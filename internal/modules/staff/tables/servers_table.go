package tables

import (
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// ServersTable backs the DataTable on /admin/servers/table. It is read-only
// (no row actions) and model-driven, so the declared-column projection SELECTs
// only the safe columns below — the secret server columns (private_key,
// public_key, host_key, user_public_key, password, database_password,
// provider_data, launch_token, …) are never declared and therefore can't reach
// /meta or /data.
type ServersTable struct{}

// NewServersTable builds the servers admin table. No service is wired because
// the table is read-only.
func NewServersTable() *ServersTable { return &ServersTable{} }

func (t *ServersTable) Config() table.Config {
	return table.Config{
		Name:           "servers",
		Model:          func() any { return &servermodels.Server{} },
		DefaultSort:    &table.SortClause{Column: "created_at", Direction: table.SortDesc},
		PerPageOptions: []int{15, 30, 50},
		DefaultPerPage: 15,
		Searchable:     []string{"name", "public_ipv4"},
		StickyHeader:   true,
	}
}

func (t *ServersTable) Columns() []table.Column {
	return []table.Column{
		table.NewTextColumn("name", "Name").AsSortable().AsSearchable(),
		table.NewTextColumn("provider", "Provider").AsSortable(),
		table.NewTextColumn("public_ipv4", "IP").AsSearchable(),
		table.NewBadgeColumn("status", "Status").AsSortable().Variants(serverStatusVariants()),
		table.NewDateTimeColumn("created_at", "Created").Format("2006-01-02").AsSortable(),
	}
}

func (t *ServersTable) Filters() []table.Filter { return nil }

// Actions returns nil: this is a read-only table with no row actions, so no
// action column is declared and the mutating /action route is never mounted.
func (t *ServersTable) Actions() []*table.Action { return nil }

func (t *ServersTable) EmptyState() *table.EmptyState {
	return table.NewEmptyState().
		Title("No servers yet").
		Message("Servers provisioned by teams will appear here.").
		Icon("server")
}

// serverStatusVariants maps each server status to a badge variant for the
// status pill.
func serverStatusVariants() map[string]table.Variant {
	return map[string]table.Variant{
		string(servertypes.ServerStatusRunning):            table.VariantSuccess,
		string(servertypes.ServerStatusProvisioning):       table.VariantInfo,
		string(servertypes.ServerStatusStarting):           table.VariantInfo,
		string(servertypes.ServerStatusNew):                table.VariantInfo,
		string(servertypes.ServerStatusAwaitingConnection): table.VariantWarning,
		string(servertypes.ServerStatusPaused):             table.VariantWarning,
		string(servertypes.ServerStatusStopped):            table.VariantSecondary,
		string(servertypes.ServerStatusArchived):           table.VariantSecondary,
		string(servertypes.ServerStatusDeleting):           table.VariantDestructive,
		string(servertypes.ServerStatusFailed):             table.VariantDestructive,
		string(servertypes.ServerStatusUnknown):            table.VariantOutline,
	}
}
