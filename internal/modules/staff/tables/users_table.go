package tables

import (
	"context"
	"strings"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// staffRoleString dereferences the nullable staff role into its raw string
// ("" when the user holds no staff role) so the row map carries a plain value
// — not a pointer, which would render as "<nil>"/an address.
func staffRoleString(r *string) string {
	if r == nil {
		return ""
	}
	return *r
}

// staffRoleLabels maps each staff role value to its human display label
// (e.g. "super_admin" → "Super Admin"), sourced from the enum.
func staffRoleLabels() map[string]string {
	m := make(map[string]string, len(stafftypes.AllStaffRoles()))
	for _, r := range stafftypes.AllStaffRoles() {
		m[string(r)] = r.Label()
	}
	return m
}

// UsersTable backs the DataTable on /admin/users/table. It is a pure Resolver
// table: it owns its entire data fetch by delegating to the existing staff
// service's ListUsersWithBilling, which assembles each user together with the
// teams they own and every team's current subscription status. Because it
// implements table.Resolver, the model-driven query path is skipped entirely —
// Config.Model returns nil and the declared Columns()/Filters() drive only the
// /meta schema.
//
// Each row carries a nested teams[] array (rendered by a custom frontend cell);
// the row map carries the array verbatim so the schema column stays a plain
// text column.
type UsersTable struct {
	svc *services.Service
}

// NewUsersTable wires the staff service whose ListUsersWithBilling backs Resolve
// and whose SetUserStatus/DeleteUser back the actions.
func NewUsersTable(svc *services.Service) *UsersTable {
	return &UsersTable{svc: svc}
}

func (t *UsersTable) Config() table.Config {
	return table.Config{
		Name:           "users",
		Model:          func() any { return nil }, // pure resolver: never dereferenced
		DefaultPerPage: 20,
		PerPageOptions: []int{20, 50, 100},
		StickyHeader:   true,
		// Declares the search box; Resolve honours req.Search against name/email.
		Searchable: []string{"name", "email"},
	}
}

func (t *UsersTable) Columns() []table.Column {
	return []table.Column{
		table.NewTextColumn("name", "Name").AsSortable().AsSearchable(),
		table.NewTextColumn("email", "Email").AsSortable().AsSearchable(),
		// teams is the nested teams[] array; the frontend renders it with a
		// custom cell. The row map carries the []map[string]any verbatim.
		table.NewTextColumn("teams", "Teams"),
		table.NewBadgeColumn("staff_role", "Staff role").
			Variants(map[string]table.Variant{
				string(stafftypes.StaffRoleSuperAdmin): table.VariantInfo,
				string(stafftypes.StaffRoleSupport):    table.VariantSecondary,
			}).
			Labels(staffRoleLabels()),
		table.NewBadgeColumn("status", "Status").AsSortable().
			Variants(map[string]table.Variant{
				"active":    table.VariantSuccess,
				"suspended": table.VariantDestructive,
			}).
			Labels(map[string]string{
				string(authtypes.UserStatusActive):    authtypes.UserStatusActive.Label(),
				string(authtypes.UserStatusSuspended): authtypes.UserStatusSuspended.Label(),
			}),
		table.NewDateTimeColumn("created_at", "Created").AsSortable().Format("2006-01-02"),
		table.NewActionColumn(),
	}
}

// Filters exposes a status set-filter (active / suspended). Resolve honours it
// via req.Filters["status"].
func (t *UsersTable) Filters() []table.Filter {
	return []table.Filter{
		table.NewSetFilter("status", "Status").
			Single().
			WithoutClause().
			Options([]table.FilterOption{
				{Value: "active", Label: "Active"},
				{Value: "suspended", Label: "Suspended"},
			}),
	}
}

// Actions returns the three super_admin account actions. The mutating
// /action/:name route is gated behind super_admin at mount time. Each handler
// reads the acting staff member's id from the context (set by the action route)
// so the service self-guards (can't suspend/delete yourself) apply.
func (t *UsersTable) Actions() []*table.Action {
	return []*table.Action{
		table.NewAction("suspend", "Suspend").
			AsButton().
			Variant(table.VariantDestructive).
			Icon("ban").
			Confirm(table.ActionConfirm{
				Title:   "Suspend this user?",
				Message: "They will be blocked from logging in and using the product.",
			}).
			Hidden(func(row map[string]any) bool { return row["status"] == "suspended" }).
			Handle(func(ctx context.Context, ids []string) error {
				actor := table.ActorID(ctx)
				for _, id := range ids {
					if err := t.svc.SetUserStatus(ctx, actor, id, authtypes.UserStatusSuspended); err != nil {
						return err
					}
				}
				return nil
			}),
		table.NewAction("unsuspend", "Unsuspend").
			AsButton().
			Icon("circle-check").
			Hidden(func(row map[string]any) bool { return row["status"] != "suspended" }).
			Handle(func(ctx context.Context, ids []string) error {
				actor := table.ActorID(ctx)
				for _, id := range ids {
					if err := t.svc.SetUserStatus(ctx, actor, id, authtypes.UserStatusActive); err != nil {
						return err
					}
				}
				return nil
			}),
		table.NewAction("delete", "Delete").
			AsButton().
			Variant(table.VariantDestructive).
			Icon("trash").
			Confirm(table.ActionConfirm{
				Title:   "Delete this user?",
				Message: "Permanently deletes the user and all their data. Only allowed if they never had a paying subscription.",
			}).
			Handle(func(ctx context.Context, ids []string) error {
				actor := table.ActorID(ctx)
				for _, id := range ids {
					if err := t.svc.DeleteUser(ctx, actor, id); err != nil {
						return err
					}
				}
				return nil
			}),
	}
}

func (t *UsersTable) EmptyState() *table.EmptyState {
	return table.NewEmptyState().
		Title("No users").
		Message("Customer accounts will appear here.").
		Icon("users")
}

// Resolve owns the full data fetch by delegating to ListUsersWithBilling. Each
// row carries a nested teams[] array (id, name, personal_team, and a folded-in
// subscription status). The declared columns/filters drive only the /meta
// schema (via table.Render).
func (t *UsersTable) Resolve(ctx context.Context, req table.Request) (*table.TableResponse, error) {
	perPage := req.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	sortColumn, sortDir := parseSort(req.Sort)
	users, total, err := t.svc.ListUsersWithBilling(ctx, repositories.ListUsersOptions{
		Limit:      perPage,
		Offset:     offset,
		Search:     req.Search,
		Status:     extractStatusFilter(req),
		SortColumn: sortColumn,
		SortDir:    sortDir,
	})
	if err != nil {
		return nil, err
	}

	rows := make([]map[string]any, 0, len(users))
	for i := range users {
		u := users[i]

		teams := make([]map[string]any, 0, len(u.Teams))
		for _, tm := range u.Teams {
			var sub map[string]any
			if tm.Subscription != nil {
				sub = map[string]any{
					"status":        tm.Subscription.Status,
					"trial_ends_at": tm.Subscription.TrialEndsAt,
				}
			}
			teams = append(teams, map[string]any{
				"id":            tm.ID,
				"name":          tm.Name,
				"personal_team": tm.PersonalTeam,
				"subscription":  sub,
			})
		}

		row := map[string]any{
			"id":         u.ID,
			"name":       u.Name,
			"email":      u.Email,
			"staff_role": staffRoleString(u.StaffRole),
			"status":     u.Status,
			"created_at": u.CreatedAt,
			"teams":      teams,
		}

		// A Resolver table skips the model-driven transformRow, so it must
		// serialize its own per-row actions; otherwise suspend/unsuspend/delete
		// never reach the DataTable.
		if actions := table.SerializeRowActions(t, row); len(actions) > 0 {
			row["_actions"] = actions
		}

		// Apply declared-column mapping (badge {value,variant} shapes + enum
		// labels) now that the row actions — which read the raw status — have
		// been serialized.
		table.MapRow(t, row)

		rows = append(rows, row)
	}

	lastPage := int((total + int64(perPage) - 1) / int64(perPage))
	if lastPage < 1 {
		lastPage = 1
	}
	from := int64(0)
	to := int64(0)
	if total > 0 {
		from = int64((page-1)*perPage + 1)
		to = int64(page * perPage)
		if to > total {
			to = total
		}
	}

	return &table.TableResponse{
		Meta: table.Render(t),
		Data: rows,
		Pagination: table.PaginationData{
			Type:        table.PaginationFull,
			CurrentPage: page,
			LastPage:    lastPage,
			PerPage:     perPage,
			Total:       total,
			From:        from,
			To:          to,
		},
	}, nil
}

// parseSort splits the framework's "column:direction" sort token into its
// parts. An empty or malformed token yields ("", "") so the repository applies
// its default (created_at desc). The column is validated downstream.
func parseSort(sort string) (column, direction string) {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return "", ""
	}
	parts := strings.SplitN(sort, ":", 2)
	column = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		direction = strings.TrimSpace(parts[1])
	}
	return column, direction
}

// extractStatusFilter reads the status set-filter out of the request, returning
// the first "active"|"suspended" value it finds, or "" to match all statuses.
func extractStatusFilter(req table.Request) string {
	clauseMap, ok := req.Filters["status"]
	if !ok {
		return ""
	}
	for _, raw := range clauseMap {
		switch v := raw.(type) {
		case string:
			if v == "active" || v == "suspended" {
				return v
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && (s == "active" || s == "suspended") {
					return s
				}
			}
		case []string:
			for _, s := range v {
				if s == "active" || s == "suspended" {
					return s
				}
			}
		}
	}
	return ""
}
