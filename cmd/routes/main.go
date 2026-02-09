package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth"
	"github.com/kkz6/launch-go/internal/modules/backup"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	wsmodule "github.com/kkz6/launch-go/internal/modules/websocket"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

var (
	methodFilter string
	pathFilter   string
	sortBy       string
	reverse      bool
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	methodStyles = map[string]lipgloss.Style{
		"GET":    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // Green
		"POST":   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")), // Orange
		"PUT":    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")),  // Blue
		"PATCH":  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),  // Cyan
		"DELETE": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")), // Red
	}

	pathStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	paramStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
)

func main() {
	flag.StringVar(&methodFilter, "method", "", "Filter routes by HTTP method (GET, POST, PUT, DELETE, etc.)")
	flag.StringVar(&pathFilter, "path", "", "Filter routes by path pattern (e.g., '/api/servers')")
	flag.StringVar(&sortBy, "sort", "path", "Sort by: path, method")
	flag.BoolVar(&reverse, "reverse", false, "Reverse the sort order")
	flag.Parse()

	routes := collectRoutes()
	routes = filterRoutes(routes)
	sortRoutes(routes)
	printRoutes(routes)
}

// RouteInfo holds information about a route
type RouteInfo struct {
	Method string
	Path   string
	Name   string
}

// collectRoutes bootstraps the app and collects all registered routes
func collectRoutes() []RouteInfo {
	// Load minimal config (only need JWT secret for auth middleware)
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create a silent logger
	logger := zerolog.Nop()

	// Create Fiber app
	fiberApp := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Create minimal app context (no DB, queue, or websocket needed for route listing)
	ctx := &app.Context{
		Deps: app.Deps{
			Config: cfg,
			Logger: &logger,
		},
	}

	// Create kernel and register modules
	kernel := app.NewKernel(&logger)
	builder := app.NewBuilderFromContext(ctx)

	// Register all modules that have routes
	kernel.
		Register(auth.NewModule(builder, nil)).
		Register(server.NewModule(builder)).
		Register(databasemodule.NewModule(builder)).
		Register(site.NewModule(builder)).
		Register(dns.NewModule(builder)).
		Register(backup.NewModule(builder)).
		Register(wsmodule.NewModule(builder))

	// Setup routes
	api := fiberApp.Group("/api")
	authMiddleware := middleware.Auth(cfg.JWT.Secret)

	// Boot all routes through the kernel
	kernel.BootHTTP(app.BootHTTPOptions{
		Router:         api,
		AuthMiddleware: authMiddleware,
	})
	kernel.BootWebhooks(fiberApp)
	kernel.BootWebSocket(api)

	// Standard HTTP methods we care about
	validMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"PATCH":  true,
		"DELETE": true,
	}

	// Collect routes from Fiber
	var routes []RouteInfo
	seen := make(map[string]bool)

	for _, route := range fiberApp.GetRoutes() {
		// Skip non-standard methods (HEAD, CONNECT, OPTIONS, TRACE)
		if !validMethods[route.Method] {
			continue
		}

		// Deduplicate routes (Fiber may register same route multiple times)
		key := route.Method + route.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		routes = append(routes, RouteInfo{
			Method: route.Method,
			Path:   route.Path,
			Name:   route.Name,
		})
	}

	return routes
}

// filterRoutes filters routes by method and path
func filterRoutes(routes []RouteInfo) []RouteInfo {
	if methodFilter == "" && pathFilter == "" {
		return routes
	}

	var filtered []RouteInfo
	for _, route := range routes {
		// Filter by method
		if methodFilter != "" && !strings.EqualFold(route.Method, methodFilter) {
			continue
		}

		// Filter by path
		if pathFilter != "" && !strings.Contains(strings.ToLower(route.Path), strings.ToLower(pathFilter)) {
			continue
		}

		filtered = append(filtered, route)
	}

	return filtered
}

// sortRoutes sorts routes by the specified field
func sortRoutes(routes []RouteInfo) {
	sort.Slice(routes, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "method":
			if routes[i].Method == routes[j].Method {
				less = routes[i].Path < routes[j].Path
			} else {
				less = routes[i].Method < routes[j].Method
			}
		default: // path
			if routes[i].Path == routes[j].Path {
				less = routes[i].Method < routes[j].Method
			} else {
				less = routes[i].Path < routes[j].Path
			}
		}

		if reverse {
			return !less
		}
		return less
	})
}

// formatMethod formats the HTTP method with color
func formatMethod(method string) string {
	style, ok := methodStyles[method]
	if !ok {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	}
	return style.Render(fmt.Sprintf("%-6s", method))
}

// formatPath formats the path with parameter highlighting
func formatPath(path string) string {
	// Highlight route parameters (e.g., :id, :serverId)
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = paramStyle.Render(part)
		} else {
			parts[i] = pathStyle.Render(part)
		}
	}
	return strings.Join(parts, pathStyle.Render("/"))
}

// printRoutes prints routes in a beautiful table
func printRoutes(routes []RouteInfo) {
	if len(routes) == 0 {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No routes found."))
		return
	}

	// Print title
	fmt.Println()
	fmt.Println(titleStyle.Render("🚀 Registered Routes"))

	// Build table rows
	rows := make([][]string, len(routes))
	for i, route := range routes {
		rows[i] = []string{
			formatMethod(route.Method),
			formatPath(route.Path),
		}
	}

	// Create table
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238"))).
		Headers("METHOD", "PATH").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	fmt.Println(t)

	// Print summary
	summary := fmt.Sprintf("Total: %d routes", len(routes))
	if methodFilter != "" || pathFilter != "" {
		filters := []string{}
		if methodFilter != "" {
			filters = append(filters, fmt.Sprintf("method=%s", methodFilter))
		}
		if pathFilter != "" {
			filters = append(filters, fmt.Sprintf("path=%s", pathFilter))
		}
		summary += fmt.Sprintf(" (filtered by %s)", strings.Join(filters, ", "))
	}
	fmt.Println(countStyle.Render(summary))
	fmt.Println()
}
