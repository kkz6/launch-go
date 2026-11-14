package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ContaboClient reads an account's existing servers. Ordering and cancellation
// are deliberately outside this client and the cloud provisioning factory.
type ContaboClient struct {
	client   *http.Client
	apiURL   string
	tokenURL string
}

func NewContaboClient() *ContaboClient {
	return &ContaboClient{
		client: &http.Client{
			Timeout: 12 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiURL:   "https://api.contabo.com/v1/compute/instances",
		tokenURL: "https://auth.contabo.com/auth/realms/contabo/protocol/openid-connect/token",
	}
}

type ContaboCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	APIUser      string `json:"api_user"`
	APIPassword  string `json:"api_password"`
}

func (c ContaboCredentials) Complete() bool {
	return c.ClientID != "" && strings.TrimSpace(c.ClientSecret) != "" &&
		c.APIUser != "" && strings.TrimSpace(c.APIPassword) != ""
}

type ContaboInstance struct {
	ID          int64  `json:"instanceId"`
	TenantID    string `json:"tenantId"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Region      string `json:"region"`
	RegionName  string `json:"regionName"`
	OSType      string `json:"osType"`
	Status      string `json:"status"`
	IPConfig    struct {
		V4 struct {
			IP string `json:"ip"`
		} `json:"v4"`
	} `json:"ipConfig"`
}

func (i ContaboInstance) Key() string {
	return i.TenantID + ":" + strconv.FormatInt(i.ID, 10)
}

func (i ContaboInstance) PublicIPv4() string {
	ip, err := netip.ParseAddr(i.IPConfig.V4.IP)
	if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return ""
	}
	return ip.String()
}

func (i ContaboInstance) CanImport() bool {
	return i.ID > 0 && i.TenantID != "" && len(i.TenantID) <= 40 &&
		i.Status == "running" && strings.EqualFold(i.OSType, "linux") && i.PublicIPv4() != ""
}

type ContaboInstancePage struct {
	Data       []ContaboInstance `json:"data"`
	Pagination struct {
		TotalPages int `json:"totalPages"`
	} `json:"_pagination"`
}

func (c *ContaboClient) ListInstances(ctx context.Context, creds ContaboCredentials, page, size int) (ContaboInstancePage, error) {
	query := url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}}
	var result ContaboInstancePage
	err := c.get(ctx, creds, c.apiURL+"?"+query.Encode(), &result)
	return result, err
}

func (c *ContaboClient) GetInstance(ctx context.Context, creds ContaboCredentials, id int64) (ContaboInstance, error) {
	var result ContaboInstancePage
	if err := c.get(ctx, creds, c.apiURL+"/"+strconv.FormatInt(id, 10), &result); err != nil {
		return ContaboInstance{}, err
	}
	if len(result.Data) != 1 || result.Data[0].ID != id {
		return ContaboInstance{}, fiberutil.NotFound("Contabo VPS not found")
	}
	return result.Data[0], nil
}

func (c *ContaboClient) get(ctx context.Context, creds ContaboCredentials, endpoint string, result any) error {
	if !creds.Complete() {
		return fiberutil.BadRequest("Contabo client ID, client secret, API username, and API password are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	form := url.Values{
		"grant_type": {"password"}, "client_id": {creds.ClientID}, "client_secret": {creds.ClientSecret},
		"username": {creds.APIUser}, "password": {creds.APIPassword},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return contaboUnavailable()
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := c.send(req, &token); err != nil {
		return err
	}
	if token.AccessToken == "" {
		return contaboUnavailable()
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return contaboUnavailable()
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("X-Request-ID", utils.UUIDv4())
	return c.send(req, result)
}

func (c *ContaboClient) send(req *http.Request, result any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return contaboUnavailable()
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(result); err != nil {
			return contaboUnavailable()
		}
		return nil
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return fiberutil.BadRequest("Contabo denied access. Check the API credentials and instance read permissions.")
	case http.StatusNotFound:
		return fiberutil.NotFound("Contabo resource not found")
	case http.StatusTooManyRequests:
		return fiberutil.TooManyRequests("Contabo is rate limiting requests. Try again shortly.")
	default:
		return contaboUnavailable()
	}
}

func contaboUnavailable() error {
	return fiber.NewError(fiber.StatusBadGateway, "Could not reach the Contabo API. Try again shortly.")
}
