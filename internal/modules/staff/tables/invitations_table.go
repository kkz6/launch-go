// Package tables defines the declarative back-office tables for the staff
// module (e.g. /admin/invitations/table).
package tables

import (
	"context"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// planLabels maps each plan id to its display name (e.g. "hobby" → "Hobby
// Plan"), sourced from the billing config so the table stays in sync.
func planLabels() map[string]string {
	plans := billingmodels.DefaultPlans()
	m := make(map[string]string, len(plans))
	for _, p := range plans {
		m[p.ID] = p.Name
	}
	return m
}

// InvitationsTable backs the DataTable on /admin/invitations. Only declared
// columns are selected and returned, so the secret invite token (which is
// never declared) can't leak through /data. The "Accepted" column surfaces
// accepted_at as a date (empty while the invite is still pending).
type InvitationsTable struct {
	svc *services.Service
}

// NewInvitationsTable wires the staff service used by the revoke action.
func NewInvitationsTable(svc *services.Service) *InvitationsTable {
	return &InvitationsTable{svc: svc}
}

func (t *InvitationsTable) Config() table.Config {
	return table.Config{
		Name:           "invitations",
		Model:          func() any { return &authmodels.PlatformInvitation{} },
		DefaultSort:    &table.SortClause{Column: "expires_at", Direction: table.SortDesc},
		PerPageOptions: []int{15, 30, 50},
		DefaultPerPage: 15,
		Searchable:     []string{"email"},
		StickyHeader:   true,
	}
}

func (t *InvitationsTable) Columns() []table.Column {
	return []table.Column{
		table.NewTextColumn("email", "Email").AsSortable().AsSearchable(),
		table.NewTextColumn("plan_id", "Plan").Labels(planLabels()),
		table.NewDateTimeColumn("trial_ends_at", "Trial ends").Format("2006-01-02").AsSortable(),
		table.NewDateTimeColumn("expires_at", "Invite expires").Format("2006-01-02").AsSortable(),
		table.NewDateTimeColumn("accepted_at", "Accepted").Format("2006-01-02"),
		table.NewActionColumn(),
	}
}

func (t *InvitationsTable) Filters() []table.Filter { return nil }

func (t *InvitationsTable) Actions() []*table.Action {
	return []*table.Action{
		table.NewAction("revoke", "Revoke").AsButton().
			Variant(table.VariantDestructive).
			Icon("trash").
			Confirm(table.ActionConfirm{
				Title:   "Revoke this invitation?",
				Message: "The invite link will stop working.",
			}).
			Handle(func(ctx context.Context, ids []string) error {
				for _, id := range ids {
					if err := t.svc.RevokeInvitation(ctx, id); err != nil {
						return err
					}
				}
				return nil
			}),
	}
}

func (t *InvitationsTable) EmptyState() *table.EmptyState {
	return table.NewEmptyState().
		Title("No invitations yet").
		Message("Invite someone from the platform admin to see them listed here.").
		Icon("mail")
}
