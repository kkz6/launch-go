package table

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func TestLocalizeTableMetaTranslatesOnlyDisplayCopy(t *testing.T) {
	trueLabel := "Active"
	tooltip := "Delete"
	input := TableMeta{
		Columns: []ColumnSerialized{{
			Type: "boolean", Key: "status", Header: "Status", TrueLabel: &trueLabel,
			Meta: map[string]any{"user_value": "Search"},
		}},
		Filters: []FilterSerialized{{
			Key: "status", Label: "Status", Type: "set",
			Options: []FilterOption{{Value: "active", Label: "Active"}},
		}},
		Actions: TableActions{Row: []ActionSerialized{{
			Name: "delete", Label: "Delete", Icon: stringPointer("trash"), Tooltip: &tooltip,
			URL: stringPointer("/users/1"),
			Confirm: &ActionConfirm{
				Title: "Delete this user?", Message: "This action cannot be undone.", ConfirmLabel: "Delete", CancelLabel: "Cancel",
			},
		}}},
		Search: TableSearchMeta{Enabled: true, Placeholder: "Search"},
		Views:  []ViewSerialized{{ID: "view-1", Title: "Search"}},
		EmptyState: &EmptyStateSerialized{
			Title: "No users", Message: "Customer accounts will appear here.", Icon: "users",
			Action: &EmptyStateAction{Label: "Create", URL: "/users/new"},
		},
	}

	var localized TableMeta
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		i18n.SetDetectedLocale(c, i18n.LocaleJapanese)
		localized = localizeTableMeta(c, input)
		return c.SendStatus(fiber.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	assert.NotEqual(t, "Status", localized.Columns[0].Header)
	assert.NotEqual(t, "Active", *localized.Columns[0].TrueLabel)
	assert.NotEqual(t, "Status", localized.Filters[0].Label)
	assert.Equal(t, "active", localized.Filters[0].Options[0].Value)
	assert.NotEqual(t, "Active", localized.Filters[0].Options[0].Label)
	assert.Equal(t, "delete", localized.Actions.Row[0].Name)
	assert.Equal(t, "/users/1", *localized.Actions.Row[0].URL)
	assert.Equal(t, "trash", *localized.Actions.Row[0].Icon)
	assert.NotEqual(t, "Delete", localized.Actions.Row[0].Label)
	assert.NotEqual(t, "Delete this user?", localized.Actions.Row[0].Confirm.Title)
	assert.NotEqual(t, "Search", localized.Search.Placeholder)
	assert.NotEqual(t, "No users", localized.EmptyState.Title)
	assert.Equal(t, "/users/new", localized.EmptyState.Action.URL)

	// User-owned view titles and arbitrary meta values must never be translated.
	assert.Equal(t, "Search", localized.Views[0].Title)
	assert.Equal(t, "Search", localized.Columns[0].Meta.(map[string]any)["user_value"])
	// Copying prevents request-specific translations from contaminating future requests.
	assert.Equal(t, "Status", input.Columns[0].Header)
	assert.Equal(t, "Delete", input.Actions.Row[0].Label)
}

func stringPointer(value string) *string { return &value }
