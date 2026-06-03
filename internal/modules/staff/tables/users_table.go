package tables

import (
	"context"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

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
	}
}

func (t *UsersTable) Columns() []table.Column {
	return []table.Column{
		table.NewTextColumn("name", "Name"),
		table.NewTextColumn("email", "Email"),
		// teams is the nested teams[] array; the frontend renders it with a
		// custom cell. The row map carries the []map[string]any verbatim.
		table.NewTextColumn("teams", "Teams"),
		table.NewBadgeColumn("staff_role", "Staff role"),
		table.NewBadgeColumn("status", "Status").Variants(map[string]table.Variant{
			"active":    table.VariantSuccess,
			"suspended": table.VariantDestructive,
		}),
		table.NewDateTimeColumn("created_at", "Created").Format("2006-01-02"),
		table.NewActionColumn(),
	}
}

// Filters returns nil: the users table has no toolbar filters yet.
func (t *UsersTable) Filters() []table.Filter { return nil }

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

	users, total, err := t.svc.ListUsersWithBilling(ctx, perPage, offset)
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
			"staff_role": u.StaffRole,
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
