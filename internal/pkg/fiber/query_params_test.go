package fiber

import (
	"net/http/httptest"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestParseIntParam(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		config   IntParamConfig
		expected int
	}{
		{
			name:  "returns default when missing",
			query: "",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
			},
			expected: 10,
		},
		{
			name:  "parses valid integer",
			query: "?count=50",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
			},
			expected: 50,
		},
		{
			name:  "returns default for invalid integer",
			query: "?count=abc",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
			},
			expected: 10,
		},
		{
			name:  "clamps to min",
			query: "?count=-5",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
				Min:     1,
			},
			expected: 1,
		},
		{
			name:  "clamps to max",
			query: "?count=200",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
				Max:     100,
			},
			expected: 100,
		},
		{
			name:  "respects both min and max",
			query: "?count=50",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
				Min:     1,
				Max:     100,
			},
			expected: 50,
		},
		{
			name:  "zero value passes when min is zero",
			query: "?count=0",
			config: IntParamConfig{
				Name:    "count",
				Default: 10,
				Min:     0,
				Max:     100,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseIntParam(c, tt.config)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		defaultVal int
		maxVal     int
		expected   int
	}{
		{
			name:       "returns default when missing",
			query:      "",
			defaultVal: 25,
			maxVal:     100,
			expected:   25,
		},
		{
			name:       "parses valid limit",
			query:      "?limit=50",
			defaultVal: 25,
			maxVal:     100,
			expected:   50,
		},
		{
			name:       "clamps to max",
			query:      "?limit=200",
			defaultVal: 25,
			maxVal:     100,
			expected:   100,
		},
		{
			name:       "clamps to min of 1",
			query:      "?limit=0",
			defaultVal: 25,
			maxVal:     100,
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseLimit(c, tt.defaultVal, tt.maxVal)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTail(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "returns default 100 when missing",
			query:    "",
			expected: 100,
		},
		{
			name:     "parses valid tail",
			query:    "?tail=500",
			expected: 500,
		},
		{
			name:     "clamps to max 10000",
			query:    "?tail=20000",
			expected: 10000,
		},
		{
			name:     "clamps to min 1",
			query:    "?tail=0",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseTail(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		defaultVal int
		expected   int
	}{
		{
			name:       "returns default when missing",
			query:      "",
			defaultVal: 5,
			expected:   5,
		},
		{
			name:       "parses valid interval",
			query:      "?interval=30",
			defaultVal: 5,
			expected:   30,
		},
		{
			name:       "clamps to max 60",
			query:      "?interval=100",
			defaultVal: 5,
			expected:   60,
		},
		{
			name:       "clamps to min 1",
			query:      "?interval=0",
			defaultVal: 5,
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseInterval(c, tt.defaultVal)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePage(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "returns default 1 when missing",
			query:    "",
			expected: 1,
		},
		{
			name:     "parses valid page",
			query:    "?page=5",
			expected: 5,
		},
		{
			name:     "clamps to min 1",
			query:    "?page=0",
			expected: 1,
		},
		{
			name:     "clamps negative to min 1",
			query:    "?page=-5",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParsePage(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePerPage(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "returns default 15 when missing",
			query:    "",
			expected: 15,
		},
		{
			name:     "parses valid per_page",
			query:    "?per_page=50",
			expected: 50,
		},
		{
			name:     "clamps to max 100",
			query:    "?per_page=200",
			expected: 100,
		},
		{
			name:     "clamps to min 1",
			query:    "?per_page=0",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParsePerPage(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected PaginationParams
	}{
		{
			name:  "returns defaults when missing",
			query: "",
			expected: PaginationParams{
				Page:    1,
				PerPage: 15,
				Offset:  0,
			},
		},
		{
			name:  "calculates offset correctly",
			query: "?page=3&per_page=20",
			expected: PaginationParams{
				Page:    3,
				PerPage: 20,
				Offset:  40, // (3-1) * 20
			},
		},
		{
			name:  "handles page 1",
			query: "?page=1&per_page=10",
			expected: PaginationParams{
				Page:    1,
				PerPage: 10,
				Offset:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result PaginationParams

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParsePagination(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		paramName  string
		defaultVal bool
		expected   bool
	}{
		{
			name:       "returns default when missing",
			query:      "",
			paramName:  "active",
			defaultVal: false,
			expected:   false,
		},
		{
			name:       "parses true",
			query:      "?active=true",
			paramName:  "active",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "parses 1 as true",
			query:      "?active=1",
			paramName:  "active",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "parses yes as true",
			query:      "?active=yes",
			paramName:  "active",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "parses false",
			query:      "?active=false",
			paramName:  "active",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "parses 0 as false",
			query:      "?active=0",
			paramName:  "active",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "parses no as false",
			query:      "?active=no",
			paramName:  "active",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "returns default for invalid value",
			query:      "?active=invalid",
			paramName:  "active",
			defaultVal: true,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result bool

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseBool(c, tt.paramName, tt.defaultVal)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		paramName  string
		defaultVal string
		expected   string
	}{
		{
			name:       "returns default when missing",
			query:      "",
			paramName:  "status",
			defaultVal: "active",
			expected:   "active",
		},
		{
			name:       "parses value",
			query:      "?status=pending",
			paramName:  "status",
			defaultVal: "active",
			expected:   "pending",
		},
		{
			name:       "handles empty string default",
			query:      "",
			paramName:  "filter",
			defaultVal: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result string

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseString(c, tt.paramName, tt.defaultVal)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "returns default 0 when missing",
			query:    "",
			expected: 0,
		},
		{
			name:     "parses valid offset",
			query:    "?offset=50",
			expected: 50,
		},
		{
			name:     "allows zero",
			query:    "?offset=0",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result int

			app.Get("/test", func(c *gofiber.Ctx) error {
				result = ParseOffset(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			_, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIntValue(t *testing.T) {
	tests := []struct {
		name       string
		str        string
		defaultVal int
		min        int
		max        int
		expected   int
	}{
		{
			name:       "returns default for empty string",
			str:        "",
			defaultVal: 10,
			min:        1,
			max:        100,
			expected:   10,
		},
		{
			name:       "parses valid integer",
			str:        "50",
			defaultVal: 10,
			min:        1,
			max:        100,
			expected:   50,
		},
		{
			name:       "returns default for invalid string",
			str:        "abc",
			defaultVal: 10,
			min:        1,
			max:        100,
			expected:   10,
		},
		{
			name:       "clamps to min",
			str:        "0",
			defaultVal: 10,
			min:        1,
			max:        100,
			expected:   1,
		},
		{
			name:       "clamps to max",
			str:        "200",
			defaultVal: 10,
			min:        1,
			max:        100,
			expected:   100,
		},
		{
			name:       "no clamping when min and max are zero",
			str:        "500",
			defaultVal: 10,
			min:        0,
			max:        0,
			expected:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseIntValue(tt.str, tt.defaultVal, tt.min, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTailValue(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		expected int
	}{
		{
			name:     "returns default 100 for empty string",
			str:      "",
			expected: 100,
		},
		{
			name:     "parses valid tail",
			str:      "500",
			expected: 500,
		},
		{
			name:     "clamps to min 1",
			str:      "0",
			expected: 1,
		},
		{
			name:     "clamps to max 10000",
			str:      "20000",
			expected: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTailValue(tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIntervalValue(t *testing.T) {
	tests := []struct {
		name       string
		str        string
		defaultVal int
		expected   int
	}{
		{
			name:       "returns default for empty string",
			str:        "",
			defaultVal: 5,
			expected:   5,
		},
		{
			name:       "parses valid interval",
			str:        "30",
			defaultVal: 5,
			expected:   30,
		},
		{
			name:       "clamps to min 1",
			str:        "0",
			defaultVal: 5,
			expected:   1,
		},
		{
			name:       "clamps to max 60",
			str:        "100",
			defaultVal: 5,
			expected:   60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseIntervalValue(tt.str, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}
