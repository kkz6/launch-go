package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0057_06_03_000000_add_staff_role_and_impersonation",
		Name:      "Add users.staff_role, impersonation_sessions, seed existing admins",
		Timestamp: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		Up:        addStaffRoleAndImpersonationUp,
	})
}

// impersonationSessionMigration is the audit log of every "spectate as user"
// session. Append-only: a row is written on start, ended_at stamped on exit.
// FKs on staff_id/target_user_id/team_id are intentionally omitted: an audit
// record must survive deletion of the user or team it references.
type impersonationSessionMigration struct {
	ID           string     `gorm:"type:char(26);primaryKey"`
	StaffID      string     `gorm:"column:staff_id;type:char(26);not null;index"`
	TargetUserID string     `gorm:"column:target_user_id;type:char(26);not null;index"`
	TeamID       *string    `gorm:"column:team_id;type:char(26)"`
	StartedAt    time.Time  `gorm:"column:started_at;type:timestamp;not null"`
	EndedAt      *time.Time `gorm:"column:ended_at;type:timestamp null"`
	Reason       *string    `gorm:"column:reason;type:text"`
}

func (impersonationSessionMigration) TableName() string { return "impersonation_sessions" }

func addStaffRoleAndImpersonationUp(db *gorm.DB) error {
	// 1. Add nullable staff_role to users (null = ordinary customer).
	if err := db.Exec(
		"ALTER TABLE users ADD COLUMN staff_role VARCHAR(20) NULL",
	).Error; err != nil {
		return err
	}

	// 2. Create impersonation_sessions.
	if err := db.Migrator().CreateTable(&impersonationSessionMigration{}); err != nil {
		return err
	}

	// 3. Seed: anyone currently holding the Spatie global admin/manager role
	//    becomes super_admin so nobody loses back-office access at cutover.
	//    Guarded so it is a no-op if the Spatie tables are already gone.
	//
	//    model_type is stored with SINGLE backslashes (e.g.
	//    "Modules\Auth\Models\User"). Postgres runs with
	//    standard_conforming_strings = on, so backslashes inside a normal
	//    single-quoted literal are taken verbatim. In a Go raw string literal
	//    (backticks) every character is literal too, so a single backslash
	//    here sends a single backslash to Postgres and matches the stored
	//    rows. Using `\\` would send two backslashes and match zero rows.
	if db.Migrator().HasTable("model_has_roles") {
		if err := db.Exec(`
			UPDATE users SET staff_role = 'super_admin'
			WHERE id IN (
				SELECT mhr.model_id FROM model_has_roles mhr
				JOIN roles r ON r.id = mhr.role_id
				WHERE r.name IN ('admin', 'manager')
				AND mhr.model_type IN ('Modules\Auth\Models\User', 'App\Models\User')
			)`).Error; err != nil {
			return err
		}
	}
	return nil
}
