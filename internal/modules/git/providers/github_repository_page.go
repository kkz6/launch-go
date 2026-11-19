package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GetUserInstallationRepositoriesPage returns one page without fetching later
// pages, so a repository review can render verified results progressively.
func (p *GitHubProvider) GetUserInstallationRepositoriesPage(ctx context.Context, token, installationID string, page int) ([]map[string]any, int, error) {
	if !decimalID(installationID) || page < 1 || page > 10000 {
		return nil, 0, errors.New("invalid GitHub repository page")
	}
	path := "/user/installations/" + installationID + "/repositories"
	response, err := p.DoRaw(ctx, http.MethodGet, path+"?per_page=100&page="+strconv.Itoa(page), token, nil)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, 0, githubUserResponseError(response)
	}
	var result struct {
		Repositories []map[string]any `json:"repositories"`
	}
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, 0, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, 0, errors.New("invalid GitHub repository response")
	}
	if result.Repositories == nil {
		return nil, 0, errors.New("invalid GitHub repository response")
	}
	for _, repository := range result.Repositories {
		id := ExtractFloatID(repository, "id")
		fullName, _ := repository["full_name"].(string)
		if !decimalID(id) || strings.TrimLeft(id, "0") == "" || strings.TrimSpace(fullName) == "" {
			return nil, 0, errors.New("invalid GitHub repository response")
		}
	}
	next, err := repositoryNextPage(response.Header.Get("Link"), p.APIURL()+path, page)
	if err != nil {
		return nil, 0, err
	}
	return availableUserRepositories(result.Repositories), next, nil
}

// GitHub's repository schema marks disabled repositories explicitly. This
// denial overrides cached user admin permissions; archived repositories remain
// eligible because archiving alone does not remove read access.
func availableUserRepositories(repositories []map[string]any) []map[string]any {
	available := make([]map[string]any, 0, len(repositories))
	for _, repository := range repositories {
		if disabled, _ := repository["disabled"].(bool); !disabled {
			available = append(available, repository)
		}
	}
	return available
}

// Only return a page number. Subsequent requests reconstruct the fixed endpoint
// instead of following a provider-supplied URL or carrying over query fields.
func repositoryNextPage(header, endpoint string, page int) (int, error) {
	invalid := errors.New("invalid GitHub repository pagination")
	expected, err := url.Parse(endpoint)
	if err != nil {
		return 0, invalid
	}
	next := 0
	for _, entry := range strings.Split(header, ",") {
		parts := strings.Split(entry, ";")
		hasNext := false
		for _, parameter := range parts[1:] {
			name, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || name != "rel" {
				continue
			}
			for _, relation := range strings.Fields(strings.Trim(strings.TrimSpace(value), `"`)) {
				if relation == "next" {
					hasNext = true
				}
			}
		}
		if !hasNext {
			continue
		}
		raw := strings.TrimSpace(parts[0])
		if next != 0 || !strings.HasPrefix(raw, "<") || !strings.HasSuffix(raw, ">") {
			return 0, invalid
		}
		link, err := url.Parse(raw[1 : len(raw)-1])
		if err != nil || link.User != nil || link.Opaque != "" || link.RawPath != "" || link.Fragment != "" || link.Path != expected.Path {
			return 0, invalid
		}
		if (link.Scheme != "" || link.Host != "") && (link.Scheme != expected.Scheme || link.Host != expected.Host) {
			return 0, invalid
		}
		query, err := url.ParseQuery(link.RawQuery)
		if err != nil || len(query["page"]) != 1 || !decimalID(query.Get("page")) {
			return 0, invalid
		}
		for key, values := range query {
			if key != "page" && (key != "per_page" || len(values) != 1 || values[0] != "100") {
				return 0, invalid
			}
		}
		next, err = strconv.Atoi(query.Get("page"))
		if err != nil || next != page+1 || next > 10000 {
			return 0, invalid
		}
	}
	return next, nil
}
