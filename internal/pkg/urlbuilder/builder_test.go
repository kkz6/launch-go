package urlbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("creates builder with base URL", func(t *testing.T) {
		b := New("https://api.example.com")
		assert.Equal(t, "https://api.example.com", b.String())
	})

	t.Run("trims trailing slash", func(t *testing.T) {
		b := New("https://api.example.com/")
		assert.Equal(t, "https://api.example.com", b.String())
	})
}

func TestBuilder_Path(t *testing.T) {
	t.Run("appends single path segment", func(t *testing.T) {
		url := New("https://api.example.com").Path("users").String()
		assert.Equal(t, "https://api.example.com/users", url)
	})

	t.Run("appends multiple path segments", func(t *testing.T) {
		url := New("https://api.example.com").Path("repos", "owner", "repo", "hooks").String()
		assert.Equal(t, "https://api.example.com/repos/owner/repo/hooks", url)
	})

	t.Run("chains path calls", func(t *testing.T) {
		url := New("https://api.example.com").Path("v1").Path("users").Path("123").String()
		assert.Equal(t, "https://api.example.com/v1/users/123", url)
	})
}

func TestBuilder_Pathf(t *testing.T) {
	t.Run("formats path segment", func(t *testing.T) {
		url := New("https://api.example.com").Pathf("users/%s", "john").String()
		assert.Equal(t, "https://api.example.com/users/john", url)
	})

	t.Run("combines with Path", func(t *testing.T) {
		url := New("https://api.example.com").Path("api").Pathf("v%d", 2).Path("users").String()
		assert.Equal(t, "https://api.example.com/api/v2/users", url)
	})
}

func TestBuilder_Query(t *testing.T) {
	t.Run("adds single query param", func(t *testing.T) {
		url := New("https://api.example.com").Path("users").Query("page", "1").String()
		assert.Equal(t, "https://api.example.com/users?page=1", url)
	})

	t.Run("adds multiple query params", func(t *testing.T) {
		url := New("https://api.example.com").
			Path("users").
			Query("page", "1").
			Query("per_page", "10").
			String()
		assert.Contains(t, url, "page=1")
		assert.Contains(t, url, "per_page=10")
	})

	t.Run("encodes special characters", func(t *testing.T) {
		url := New("https://api.example.com").Query("q", "hello world").String()
		assert.Contains(t, url, "q=hello+world")
	})
}

func TestBuilder_QueryIf(t *testing.T) {
	t.Run("adds param when condition is true", func(t *testing.T) {
		url := New("https://api.example.com").QueryIf(true, "active", "true").String()
		assert.Contains(t, url, "active=true")
	})

	t.Run("skips param when condition is false", func(t *testing.T) {
		url := New("https://api.example.com").QueryIf(false, "active", "true").String()
		assert.NotContains(t, url, "active")
	})
}

func TestBuilder_QueryIfNotEmpty(t *testing.T) {
	t.Run("adds param when value is not empty", func(t *testing.T) {
		url := New("https://api.example.com").QueryIfNotEmpty("search", "term").String()
		assert.Contains(t, url, "search=term")
	})

	t.Run("skips param when value is empty", func(t *testing.T) {
		url := New("https://api.example.com").QueryIfNotEmpty("search", "").String()
		assert.NotContains(t, url, "search")
		assert.NotContains(t, url, "?")
	})
}

func TestBuilder_QueryMap(t *testing.T) {
	t.Run("adds all params from map", func(t *testing.T) {
		url := New("https://api.example.com").QueryMap(map[string]string{
			"page":     "1",
			"per_page": "10",
		}).String()
		assert.Contains(t, url, "page=1")
		assert.Contains(t, url, "per_page=10")
	})

	t.Run("handles empty map", func(t *testing.T) {
		url := New("https://api.example.com").QueryMap(map[string]string{}).String()
		assert.Equal(t, "https://api.example.com", url)
	})
}

func TestBuilder_WithSuffix(t *testing.T) {
	t.Run("adds suffix to path", func(t *testing.T) {
		url := New("https://api.example.com").Path("data").WithSuffix(".json").String()
		assert.Equal(t, "https://api.example.com/data.json", url)
	})

	t.Run("suffix before query params", func(t *testing.T) {
		url := New("https://api.example.com").
			Path("data").
			WithSuffix(".json").
			Query("format", "pretty").
			String()
		assert.Equal(t, "https://api.example.com/data.json?format=pretty", url)
	})
}

func TestBuilder_Clone(t *testing.T) {
	t.Run("creates independent copy", func(t *testing.T) {
		base := New("https://api.example.com").Path("v1")
		clone := base.Clone()

		// Modify clone
		clone.Path("users")

		// Original should be unchanged
		assert.Equal(t, "https://api.example.com/v1", base.String())
		assert.Equal(t, "https://api.example.com/v1/users", clone.String())
	})

	t.Run("clones query params", func(t *testing.T) {
		base := New("https://api.example.com").Query("token", "abc")
		clone := base.Clone()

		clone.Query("extra", "value")

		assert.NotContains(t, base.String(), "extra")
		assert.Contains(t, clone.String(), "extra=value")
		assert.Contains(t, clone.String(), "token=abc")
	})
}

func TestCommonBuilders(t *testing.T) {
	t.Run("GitHub builder", func(t *testing.T) {
		url := GitHub.Clone().Path("repos", "owner", "repo").String()
		assert.Equal(t, "https://api.github.com/repos/owner/repo", url)
	})

	t.Run("GitLab builder", func(t *testing.T) {
		url := GitLab.Clone().Path("projects", "123").String()
		assert.Equal(t, "https://gitlab.com/api/v4/projects/123", url)
	})

	t.Run("DigitalOcean builder", func(t *testing.T) {
		url := DigitalOcean.Clone().Path("droplets").String()
		assert.Equal(t, "https://api.digitalocean.com/v2/droplets", url)
	})
}

func TestWebhook(t *testing.T) {
	t.Run("constructs webhook URL", func(t *testing.T) {
		url := Webhook("https://app.example.com", "github", "site123")
		assert.Equal(t, "https://app.example.com/api/webhooks/github/site123", url)
	})
}

func TestTaskCallback(t *testing.T) {
	t.Run("constructs task callback URL", func(t *testing.T) {
		url := TaskCallback("https://app.example.com", "task456")
		assert.Equal(t, "https://app.example.com/api/tasks/task456/callback", url)
	})
}

func TestAPI(t *testing.T) {
	t.Run("constructs API URL", func(t *testing.T) {
		url := API("https://app.example.com", "servers", "srv123", "status")
		assert.Equal(t, "https://app.example.com/api/servers/srv123/status", url)
	})

	t.Run("handles single segment", func(t *testing.T) {
		url := API("https://app.example.com", "health")
		assert.Equal(t, "https://app.example.com/api/health", url)
	})
}

func TestBuilder_HTTPSchemes(t *testing.T) {
	t.Run("preserves https scheme", func(t *testing.T) {
		url := New("https://example.com").Path("api", "v1").String()
		assert.True(t, len(url) > 8 && url[:8] == "https://")
	})

	t.Run("preserves http scheme", func(t *testing.T) {
		url := New("http://localhost:8080").Path("api").String()
		assert.True(t, len(url) > 7 && url[:7] == "http://")
	})
}

func TestBuilder_RealWorldExamples(t *testing.T) {
	t.Run("GitHub create webhook", func(t *testing.T) {
		owner := "octocat"
		repo := "hello-world"
		url := GitHub.Clone().Path("repos", owner, repo, "hooks").String()
		assert.Equal(t, "https://api.github.com/repos/octocat/hello-world/hooks", url)
	})

	t.Run("DigitalOcean list droplets with pagination", func(t *testing.T) {
		url := DigitalOcean.Clone().
			Path("droplets").
			Query("page", "2").
			Query("per_page", "50").
			String()
		assert.Contains(t, url, "droplets")
		assert.Contains(t, url, "page=2")
		assert.Contains(t, url, "per_page=50")
	})

	t.Run("GitLab project with encoded path", func(t *testing.T) {
		projectID := "123"
		url := GitLab.Clone().Path("projects", projectID, "repository", "branches").String()
		assert.Equal(t, "https://gitlab.com/api/v4/projects/123/repository/branches", url)
	})
}
