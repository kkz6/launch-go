// Package urlbuilder provides a fluent API for constructing URLs.
// It eliminates scattered fmt.Sprintf patterns for URL construction
// throughout the codebase.
package util

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Builder provides a fluent interface for constructing URLs.
type Builder struct {
	base   string
	paths  []string
	query  url.Values
	suffix string
}

// New creates a new URL builder with the given base URL.
//
// Example:
//
//	builder := util.New("https://api.github.com")
func New(baseURL string) *Builder {
	return &Builder{
		base:  strings.TrimRight(baseURL, "/"),
		query: make(url.Values),
	}
}

// Path appends path segments to the URL.
// Each segment is URL-encoded if necessary.
//
// Example:
//
//	builder.Path("repos", owner, repo, "hooks")
//	// -> https://api.github.com/repos/owner/repo/hooks
func (b *Builder) Path(segments ...string) *Builder {
	b.paths = append(b.paths, segments...)
	return b
}

// Pathf appends a formatted path segment.
//
// Example:
//
//	builder.Pathf("users/%s/repos", username)
func (b *Builder) Pathf(format string, args ...any) *Builder {
	b.paths = append(b.paths, fmt.Sprintf(format, args...))
	return b
}

// Query adds a query parameter.
//
// Example:
//
//	builder.Query("page", "1").Query("per_page", "100")
func (b *Builder) Query(key, value string) *Builder {
	b.query.Set(key, value)
	return b
}

// QueryIf adds a query parameter only if the condition is true.
//
// Example:
//
//	builder.QueryIf(page > 1, "page", strconv.Itoa(page))
func (b *Builder) QueryIf(condition bool, key, value string) *Builder {
	if condition {
		b.query.Set(key, value)
	}
	return b
}

// QueryIfNotEmpty adds a query parameter only if the value is not empty.
//
// Example:
//
//	builder.QueryIfNotEmpty("search", searchTerm)
func (b *Builder) QueryIfNotEmpty(key, value string) *Builder {
	if value != "" {
		b.query.Set(key, value)
	}
	return b
}

// QueryMap adds multiple query parameters from a map.
//
// Example:
//
//	builder.QueryMap(map[string]string{"page": "1", "sort": "name"})
func (b *Builder) QueryMap(params map[string]string) *Builder {
	for k, v := range params {
		b.query.Set(k, v)
	}
	return b
}

// WithSuffix adds a suffix to the final path (e.g., ".json").
//
// Example:
//
//	builder.Path("users", "123").WithSuffix(".json")
func (b *Builder) WithSuffix(suffix string) *Builder {
	b.suffix = suffix
	return b
}

// String builds and returns the final URL string.
func (b *Builder) String() string {
	// If no path segments, return base with optional query
	if len(b.paths) == 0 {
		result := b.base + b.suffix
		if len(b.query) > 0 {
			result += "?" + b.query.Encode()
		}
		return result
	}

	// Build the path portion
	pathStr := path.Join(b.paths...)

	// Combine base and path
	result := b.base + "/" + pathStr

	// Add suffix
	if b.suffix != "" {
		result += b.suffix
	}

	// Add query string
	if len(b.query) > 0 {
		result += "?" + b.query.Encode()
	}

	return result
}

// Clone creates a copy of the builder for branching.
//
// Example:
//
//	base := util.New("https://api.example.com").Path("v1")
//	usersURL := base.Clone().Path("users")
//	ordersURL := base.Clone().Path("orders")
func (b *Builder) Clone() *Builder {
	newQuery := make(url.Values)
	for k, v := range b.query {
		newQuery[k] = append([]string{}, v...)
	}

	pathsCopy := make([]string, len(b.paths))
	copy(pathsCopy, b.paths)

	return &Builder{
		base:   b.base,
		paths:  pathsCopy,
		query:  newQuery,
		suffix: b.suffix,
	}
}

// Common API base URLs for convenience.
var (
	// GitHub is a builder for GitHub API.
	GitHub = New("https://api.github.com")

	// GitLab is a builder for GitLab API.
	GitLab = New("https://gitlab.com/api/v4")

	// Bitbucket is a builder for Bitbucket API.
	Bitbucket = New("https://api.bitbucket.org/2.0")

	// DigitalOcean is a builder for DigitalOcean API.
	DigitalOcean = New("https://api.digitalocean.com/v2")

	// Hetzner is a builder for Hetzner API.
	Hetzner = New("https://api.hetzner.cloud/v1")

	// Vultr is a builder for Vultr API.
	Vultr = New("https://api.vultr.com/v2")

	// Linode is a builder for Linode API.
	Linode = New("https://api.linode.com/v4")
)

// App creates a builder using the application's base URL.
// This should be called with the app URL from config.
//
// Example:
//
//	callbackURL := util.App(config.AppURL).Path("api", "webhooks", "callback").String()
func App(baseURL string) *Builder {
	return New(baseURL)
}

// Webhook creates a URL for webhook endpoints.
//
// Example:
//
//	url := util.Webhook(appURL, "github", siteID)
//	// -> https://app.example.com/api/webhooks/github/site123
func Webhook(appURL, provider, id string) string {
	return New(appURL).Path("api", "webhooks", provider, id).String()
}

// TaskCallback creates a URL for task callback endpoints.
//
// Example:
//
//	url := util.TaskCallback(appURL, taskID)
//	// -> https://app.example.com/api/tasks/task123/callback
func TaskCallback(appURL, taskID string) string {
	return New(appURL).Path("api", "tasks", taskID, "callback").String()
}

// API creates a URL for internal API endpoints.
//
// Example:
//
//	url := util.API(appURL, "servers", serverID, "status")
//	// -> https://app.example.com/api/servers/server123/status
func API(appURL string, segments ...string) string {
	return New(appURL).Path(append([]string{"api"}, segments...)...).String()
}
