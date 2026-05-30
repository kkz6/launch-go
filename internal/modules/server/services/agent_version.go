package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// launchAgentRepo is the GitHub repo that publishes the launch-agent
// binary — same source the install script pulls from.
const launchAgentRepo = "kkz6/launch-util"

// agentVersionTTL bounds how often we hit the GitHub releases API. A new
// agent release is a rare event, so a generous cache keeps the update
// banner cheap while staying fresh enough.
const agentVersionTTL = 30 * time.Minute

var (
	agentVerMu      sync.Mutex
	agentVerCache   string
	agentVerExpires time.Time
	// agentVerFetch is the actual network call, kept as a package var so
	// tests can stub it without a live GitHub dependency.
	agentVerFetch = fetchLatestAgentTag
)

// latestAgentVersion returns the latest published launch-agent version
// (no leading "v"), cached in-process. On a fetch error it returns the
// last cached value (possibly "") so the banner degrades gracefully
// rather than erroring the whole server page.
func latestAgentVersion(ctx context.Context) string {
	agentVerMu.Lock()
	defer agentVerMu.Unlock()

	now := time.Now()
	if agentVerCache != "" && now.Before(agentVerExpires) {
		return agentVerCache
	}
	if v, err := agentVerFetch(ctx); err == nil && v != "" {
		agentVerCache = v
		agentVerExpires = now.Add(agentVersionTTL)
	}
	return agentVerCache
}

func fetchLatestAgentTag(ctx context.Context) (string, error) {
	url := "https://api.github.com/repos/" + launchAgentRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases/latest: status %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return normalizeAgentVersion(body.TagName), nil
}

// GetAgentVersionInfo reports the installed vs latest Launch Agent
// version for a server so the UI can render an update banner with an
// "Update" action (POST .../services/:id/update).
func (s *Service) GetAgentVersionInfo(ctx context.Context, serverID, teamID string) (*dto.AgentVersionResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	installed := ""
	serviceID := ""
	if svc, err := s.repos.Service().FindByServerAndSoftware(ctx, serverID, types.SoftwareLaunchAgent); err == nil && svc != nil {
		installed = normalizeAgentVersion(svc.Version)
		serviceID = svc.ID
	}

	latest := latestAgentVersion(ctx)

	return &dto.AgentVersionResponse{
		ServiceID:       serviceID,
		Installed:       installed,
		Latest:          latest,
		UpdateAvailable: serviceID != "" && agentUpdateAvailable(installed, latest),
	}, nil
}

// agentUpdateAvailable is true when `latest` is a known semver and
// `installed` is either older or unparseable (e.g. the stale "latest" /
// "master" placeholders that predate version injection — those should
// prompt an update so the row picks up a real version).
func agentUpdateAvailable(installed, latest string) bool {
	lp := parseSemver(latest)
	if lp == nil {
		return false // we don't actually know what the latest is
	}
	ip := parseSemver(installed)
	if ip == nil {
		return true // installed version unknown but a real release exists
	}
	for i := 0; i < 3; i++ {
		if ip[i] != lp[i] {
			return ip[i] < lp[i]
		}
	}
	return false
}

func normalizeAgentVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// parseSemver returns [major, minor, patch] or nil if v isn't a numeric
// dotted version. Pre-release/build suffixes on a part are dropped.
func parseSemver(v string) []int {
	v = normalizeAgentVersion(v)
	if v == "" {
		return nil
	}
	parts := strings.SplitN(v, ".", 3)
	out := make([]int, 3)
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			break
		}
		num := parts[i]
		if idx := strings.IndexAny(num, "-+"); idx >= 0 {
			num = num[:idx]
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}
