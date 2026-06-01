package services

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DomainService manages application domains and the Traefik dynamic-
// config writes that back them.
//
// Lives alongside ApplicationService rather than as a separate module
// because every mutation needs to reach into the application's container
// name + internal port to render the Traefik file.
type DomainService struct {
	*BaseService
}

// NewDomainService wires the service.
func NewDomainService(deps *ServiceDeps) *DomainService {
	return &DomainService{BaseService: NewBaseService(deps)}
}

// validateStoredCert enforces the one cross-field rule the struct tags
// can't express on a *string: a "stored"-provider domain needs a real
// 26-char (ULID) stored_certificate_id, while a letsencrypt domain
// ignores it entirely. The DTO tag is only `omitempty,max=26` (NOT
// len=26) on purpose: go-playground/validator's omitempty skips only
// NIL pointers, so a non-nil pointer to "" — which the UI sends for
// letsencrypt domains — used to trip `len=26` and 422 the whole
// request even though the cert id is irrelevant. The exact-length
// check lives here instead, gated on the provider actually being
// "stored". provider must be the EFFECTIVE provider (empty string when
// the caller isn't changing it on an update).
func validateStoredCert(provider string, storedID *string) error {
	if provider != "stored" {
		return nil
	}
	if storedID == nil || len(*storedID) != 26 {
		return fiberutil.Validation(
			"Pick a certificate from your library — a stored-certificate domain needs a stored_certificate_id.")
	}
	return nil
}

// ListDomains returns the live domains attached to an application,
// after validating the (server, project, app) chain.
func (s *DomainService) ListDomains(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.DomainResponse, error) {
	if _, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Domain().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DomainResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDomainResponse(&rows[i]))
	}
	return out, nil
}

// CreateDomain attaches a new hostname to the application. After the row
// is persisted we enqueue a write of the Traefik config file so the new
// route takes effect — the asynq job below dispatches a one-shot SSH
// task that re-renders the YAML.
func (s *DomainService) CreateDomain(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}

	host := strings.TrimSpace(strings.ToLower(req.Host))
	host = strings.TrimSuffix(host, ".")
	if !validHostname.MatchString(host) {
		return dto.DomainResponse{}, fiberutil.BadRequest("Invalid hostname")
	}

	taken, err := s.Repos().Domain().ExistsByHost(ctx, applicationID, host)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if taken {
		return dto.DomainResponse{}, fiberutil.Conflict("This host is already attached to the application")
	}

	https := true
	if req.HTTPS != nil {
		https = *req.HTTPS
	}
	stripPath := false
	if req.StripPath != nil {
		stripPath = *req.StripPath
	}
	certProvider := "letsencrypt"
	if req.CertificateProvider != nil && *req.CertificateProvider != "" {
		certProvider = *req.CertificateProvider
	}
	if err := validateStoredCert(certProvider, req.StoredCertificateID); err != nil {
		return dto.DomainResponse{}, err
	}

	// ApplicationID is `*string` since the model went polymorphic
	// (also backs compose domains). Take the address of a local so
	// the owner column gets set.
	ownerID := applicationID
	d := &models.ApplicationDomain{
		ApplicationID:       &ownerID,
		Host:                host,
		Path:                trimEmpty(req.Path),
		InternalPath:        trimEmpty(req.InternalPath),
		StripPath:           stripPath,
		ContainerPort:       req.ContainerPort,
		HTTPS:               https,
		CertificateProvider: certProvider,
		StoredCertificateID: req.StoredCertificateID,
	}
	if err := s.Repos().Domain().Create(ctx, d); err != nil {
		return dto.DomainResponse{}, err
	}

	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}

	resp := dto.ToDomainResponse(d)
	s.BroadcastToTeam(teamID, "docker.application.domain.added", resp)
	return *resp, nil
}

// UpdateDomain toggles HTTPS or updates the path prefix. Host changes
// aren't allowed — see UpdateDomainRequest for the reasoning.
func (s *DomainService) UpdateDomain(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if d.ApplicationID == nil || *d.ApplicationID != applicationID {
		return dto.DomainResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.HTTPS != nil {
		updates["https"] = *req.HTTPS
	}
	if req.Path != nil {
		updates["path"] = trimEmpty(req.Path)
	}
	if req.InternalPath != nil {
		updates["internal_path"] = trimEmpty(req.InternalPath)
	}
	if req.StripPath != nil {
		updates["strip_path"] = *req.StripPath
	}
	if req.ContainerPort != nil {
		// Pass nil-equivalent through when caller sends 0 — lets the
		// UI "clear the override" by sending 0 from the input.
		if *req.ContainerPort > 0 {
			updates["container_port"] = *req.ContainerPort
		} else {
			updates["container_port"] = nil
		}
	}
	if req.CertificateProvider != nil && *req.CertificateProvider != "" {
		updates["certificate_provider"] = *req.CertificateProvider
	}
	// Only enforce the stored-cert pairing when the caller is actually
	// (re)setting the provider to "stored" — leaving it unset keeps the
	// row's existing provider and shouldn't re-validate.
	{
		provider := ""
		if req.CertificateProvider != nil {
			provider = *req.CertificateProvider
		}
		if err := validateStoredCert(provider, req.StoredCertificateID); err != nil {
			return dto.DomainResponse{}, err
		}
	}
	if req.StoredCertificateID != nil {
		if *req.StoredCertificateID == "" {
			updates["stored_certificate_id"] = nil
		} else {
			updates["stored_certificate_id"] = *req.StoredCertificateID
		}
	}
	if len(updates) > 0 {
		if err := s.Repos().Domain().UpdateFields(ctx, id, updates); err != nil {
			return dto.DomainResponse{}, err
		}
	}

	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}

	reloaded, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	resp := dto.ToDomainResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.application.domain.updated", resp)
	return *resp, nil
}

// DeleteDomain soft-deletes a domain and triggers a Traefik resync.
func (s *DomainService) DeleteDomain(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.ApplicationID == nil || *d.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Domain().Delete(ctx, id); err != nil {
		return err
	}
	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}
	s.BroadcastToTeam(teamID, "docker.application.domain.deleted", map[string]any{
		"id":             id,
		"application_id": applicationID,
		"team_id":        teamID,
		"server_id":      serverID,
	})
	return nil
}

// ValidateDNS resolves the domain's hostname against public DNS and
// compares the result with the docker server's public IP. The
// frontend's "Validate DNS" button shows the user whether the
// hostname is pointing at the right server before the deploy / cert
// issuance bites them.
//
// Wildcard-DNS hostnames (*.traefik.me, *.sslip.io, *.nip.io) skip
// the lookup and report ok=true — those resolvers always answer
// with the IP encoded in the label by definition, no provisioning
// required.
func (s *DomainService) ValidateDNS(
	ctx context.Context, domainID, applicationID, projectID, serverID, teamID string,
) (dto.ValidateDNSResponse, error) {
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.ValidateDNSResponse{}, err
	}
	d, err := s.Repos().Domain().FindByID(ctx, domainID)
	if err != nil {
		return dto.ValidateDNSResponse{}, err
	}
	if d.ApplicationID == nil || *d.ApplicationID != applicationID {
		return dto.ValidateDNSResponse{}, fiberutil.NotFound()
	}
	return s.validateDNSAgainstServer(ctx, d.Host, app.ServerID)
}

// validateDNSAgainstServer is the shared DNS-lookup core used by
// ValidateDNS (app-scoped) and ValidateComposeDNS (compose-scoped).
// Wrappers handle the scope check + domain-ownership check; this only
// runs the wildcard-suffix short-circuit and the public-IP comparison.
func (s *DomainService) validateDNSAgainstServer(
	ctx context.Context, rawHost, serverID string,
) (dto.ValidateDNSResponse, error) {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	resp := dto.ValidateDNSResponse{Host: host}

	// Wildcard-DNS hostnames are routable by definition.
	for _, suffix := range wildcardDNSSuffixes {
		if strings.HasSuffix(host, suffix) {
			resp.OK = true
			resp.Wildcard = true
			resp.Message = "Wildcard DNS hostname — already routable, no validation needed."
			return resp, nil
		}
	}

	server, err := s.ServerRepos().Server().FindByID(ctx, serverID)
	if err != nil {
		return dto.ValidateDNSResponse{}, err
	}
	expectedIP := ""
	if server.PublicIPv4 != nil {
		expectedIP = *server.PublicIPv4
	}
	resp.ExpectedIP = expectedIP

	// Bounded DNS lookup so a slow resolver doesn't block the request
	// thread; 5s is generous for public A records.
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		resp.OK = false
		resp.Message = fmt.Sprintf("DNS lookup failed: %v", err)
		return resp, nil
	}
	for _, ip := range ips {
		v4 := ip.IP.To4()
		if v4 == nil {
			continue
		}
		resp.ResolvedIPs = append(resp.ResolvedIPs, v4.String())
		if expectedIP != "" && v4.String() == expectedIP {
			resp.OK = true
		}
	}
	switch {
	case resp.OK:
		resp.Message = fmt.Sprintf("Resolves to %s ✓", expectedIP)
	case len(resp.ResolvedIPs) == 0:
		resp.Message = "Hostname doesn't resolve to any A record yet."
	default:
		resp.Message = fmt.Sprintf(
			"Resolves to %s — expected %s",
			strings.Join(resp.ResolvedIPs, ", "),
			expectedIP,
		)
	}
	return resp, nil
}

// wildcardDNSSuffixes mirrors the frontend list — keep in sync with
// CreateDomain.vue's WILDCARD_DNS_SUFFIXES.
//
// Note: traefik.me is NOT here despite its suggestive name. The
// service only resolves `traefik.me` itself to 127.0.0.1 — IP-
// encoded subdomains like `1-2-3-4.traefik.me` return SERVFAIL
// from public resolvers. Including it caused user-visible
// "server IP could not be found" errors.
var wildcardDNSSuffixes = []string{
	".sslip.io",
	".nip.io",
	".localtest.me",
}

// scopedApp resolves the (server, project, application) triple inside the
// caller's team. Returns the application so the caller can dispatch
// follow-up work without an extra DB hit.
func (s *DomainService) scopedApp(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) (*models.Application, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return app, nil
}

// dispatchTraefikSync enqueues an async job that re-renders the Traefik
// config for the application. Idempotent: re-running it with the same
// inputs produces the same file content, so we can safely fire it on
// every domain mutation without worrying about ordering.
func (s *DomainService) dispatchTraefikSync(
	ctx context.Context, app *models.Application, teamID string,
) error {
	task, err := jobs.NewSyncTraefikConfigTask(app.ID, app.ServerID, teamID)
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

// dispatchComposeTraefikSync mirrors dispatchTraefikSync for compose
// stacks — dispatches the compose-side renderer job. Same idempotency
// rationale.
func (s *DomainService) dispatchComposeTraefikSync(
	ctx context.Context, c *models.Compose, teamID string,
) error {
	_ = ctx
	task, err := jobs.NewSyncComposeTraefikConfigTask(c.ID, c.ServerID, teamID)
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

// --- Compose-scoped domain operations ------------------------------
//
// Mirrors the application-domain methods but with a service-name
// requirement: every compose domain must name a YAML service to
// route to. The renderer uses (service, port) to compute the target
// container name; both are mandatory.
//
// Uniqueness, host validation, DNS validation are all shared
// behaviour — only the owner scoping + a couple of extra required-
// field checks differ.

func (s *DomainService) ListComposeDomains(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) ([]dto.DomainResponse, error) {
	if _, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Domain().ListForCompose(ctx, composeID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DomainResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDomainResponse(&rows[i]))
	}
	return out, nil
}

func (s *DomainService) CreateComposeDomain(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
	req *dto.CreateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	c, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}

	host := strings.TrimSpace(strings.ToLower(req.Host))
	host = strings.TrimSuffix(host, ".")
	if !validHostname.MatchString(host) {
		return dto.DomainResponse{}, fiberutil.BadRequest("Invalid hostname")
	}

	// service_name + container_port are mandatory on compose — the
	// renderer needs both to produce a valid `service:` block.
	serviceName := ""
	if req.ServiceName != nil {
		serviceName = strings.TrimSpace(*req.ServiceName)
	}
	if serviceName == "" {
		return dto.DomainResponse{}, fiberutil.BadRequest(
			"service_name is required for compose domains",
		)
	}
	if req.ContainerPort == nil || *req.ContainerPort <= 0 {
		return dto.DomainResponse{}, fiberutil.BadRequest(
			"container_port is required for compose domains (no fallback — the YAML owns the port)",
		)
	}

	taken, err := s.Repos().Domain().ExistsByHostForCompose(ctx, composeID, host)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if taken {
		return dto.DomainResponse{}, fiberutil.Conflict("This host is already attached to the stack")
	}

	https := true
	if req.HTTPS != nil {
		https = *req.HTTPS
	}
	stripPath := false
	if req.StripPath != nil {
		stripPath = *req.StripPath
	}
	certProvider := "letsencrypt"
	if req.CertificateProvider != nil && *req.CertificateProvider != "" {
		certProvider = *req.CertificateProvider
	}
	if err := validateStoredCert(certProvider, req.StoredCertificateID); err != nil {
		return dto.DomainResponse{}, err
	}

	ownerID := composeID
	d := &models.ApplicationDomain{
		ComposeID:           &ownerID,
		Host:                host,
		Path:                trimEmpty(req.Path),
		InternalPath:        trimEmpty(req.InternalPath),
		StripPath:           stripPath,
		ContainerPort:       req.ContainerPort,
		HTTPS:               https,
		CertificateProvider: certProvider,
		StoredCertificateID: req.StoredCertificateID,
		ServiceName:         &serviceName,
	}
	if err := s.Repos().Domain().Create(ctx, d); err != nil {
		return dto.DomainResponse{}, err
	}

	if err := s.dispatchComposeTraefikSync(ctx, c, teamID); err != nil {
		s.LogError(err, "failed to enqueue compose traefik sync", "compose_id", c.ID)
	}

	resp := dto.ToDomainResponse(d)
	s.BroadcastToTeam(teamID, "docker.compose.domain.added", resp)
	return *resp, nil
}

func (s *DomainService) UpdateComposeDomain(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
	req *dto.UpdateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	c, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if d.ComposeID == nil || *d.ComposeID != composeID {
		return dto.DomainResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.HTTPS != nil {
		updates["https"] = *req.HTTPS
	}
	if req.Path != nil {
		updates["path"] = trimEmpty(req.Path)
	}
	if req.InternalPath != nil {
		updates["internal_path"] = trimEmpty(req.InternalPath)
	}
	if req.StripPath != nil {
		updates["strip_path"] = *req.StripPath
	}
	if req.ContainerPort != nil {
		// Unlike the application path, 0/nil is not a "clear the
		// override" signal on compose — the field is required. Reject
		// the clear; treat positive as a real change.
		if *req.ContainerPort <= 0 {
			return dto.DomainResponse{}, fiberutil.BadRequest(
				"container_port is required for compose domains",
			)
		}
		updates["container_port"] = *req.ContainerPort
	}
	if req.CertificateProvider != nil && *req.CertificateProvider != "" {
		updates["certificate_provider"] = *req.CertificateProvider
	}
	// Only enforce the stored-cert pairing when the caller is actually
	// (re)setting the provider to "stored" — leaving it unset keeps the
	// row's existing provider and shouldn't re-validate.
	{
		provider := ""
		if req.CertificateProvider != nil {
			provider = *req.CertificateProvider
		}
		if err := validateStoredCert(provider, req.StoredCertificateID); err != nil {
			return dto.DomainResponse{}, err
		}
	}
	if req.StoredCertificateID != nil {
		if *req.StoredCertificateID == "" {
			updates["stored_certificate_id"] = nil
		} else {
			updates["stored_certificate_id"] = *req.StoredCertificateID
		}
	}
	if req.ServiceName != nil {
		s := strings.TrimSpace(*req.ServiceName)
		if s == "" {
			return dto.DomainResponse{}, fiberutil.BadRequest(
				"service_name cannot be cleared on a compose domain",
			)
		}
		updates["service_name"] = s
	}
	if len(updates) > 0 {
		if err := s.Repos().Domain().UpdateFields(ctx, id, updates); err != nil {
			return dto.DomainResponse{}, err
		}
	}

	if err := s.dispatchComposeTraefikSync(ctx, c, teamID); err != nil {
		s.LogError(err, "failed to enqueue compose traefik sync", "compose_id", c.ID)
	}

	reloaded, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	resp := dto.ToDomainResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.compose.domain.updated", resp)
	return *resp, nil
}

func (s *DomainService) DeleteComposeDomain(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	c, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.ComposeID == nil || *d.ComposeID != composeID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Domain().Delete(ctx, id); err != nil {
		return err
	}
	if err := s.dispatchComposeTraefikSync(ctx, c, teamID); err != nil {
		s.LogError(err, "failed to enqueue compose traefik sync", "compose_id", c.ID)
	}
	s.BroadcastToTeam(teamID, "docker.compose.domain.deleted", map[string]any{
		"id":         id,
		"compose_id": composeID,
		"team_id":    teamID,
		"server_id":  serverID,
	})
	return nil
}

// ValidateComposeDNS mirrors ValidateDNS for compose-owned domains.
// Shares the lookup core via validateDNSAgainstServer; only the scope
// + ownership check differ.
func (s *DomainService) ValidateComposeDNS(
	ctx context.Context, domainID, composeID, projectID, serverID, teamID string,
) (dto.ValidateDNSResponse, error) {
	c, err := s.scopedCompose(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.ValidateDNSResponse{}, err
	}
	d, err := s.Repos().Domain().FindByID(ctx, domainID)
	if err != nil {
		return dto.ValidateDNSResponse{}, err
	}
	if d.ComposeID == nil || *d.ComposeID != composeID {
		return dto.ValidateDNSResponse{}, fiberutil.NotFound()
	}
	return s.validateDNSAgainstServer(ctx, d.Host, c.ServerID)
}

// scopedCompose resolves (server, project, compose) inside the
// caller's team — mirrors scopedApp but on the compose repo.
func (s *DomainService) scopedCompose(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) (*models.Compose, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return c, nil
}

// validHostname accepts a minimal RFC-1035-ish hostname: labels of
// alphanumerics and hyphens (no leading/trailing hyphen), separated by
// dots, total length up to 253. Rejects underscores, schemes, and
// trailing slashes — which is most of the malformed input we've seen
// from copy-paste mistakes.
var validHostname = regexp.MustCompile(
	`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

func trimEmpty(p *string) *string {
	if p == nil {
		return nil
	}
	t := strings.TrimSpace(*p)
	if t == "" {
		return nil
	}
	return &t
}
