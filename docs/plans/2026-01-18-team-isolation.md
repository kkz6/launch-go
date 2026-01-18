# Team Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add direct `team_id` foreign key to all team-owned resources so each team operates as a completely isolated workspace.

**Architecture:** Every resource that belongs to a team will have a direct `team_id` column with NOT NULL constraint and foreign key to teams table. All queries will filter by team_id directly without needing joins through parent resources.

**Tech Stack:** Go, GORM, MySQL, Fiber

---

## Overview

### Models Requiring New `team_id` Column (10 models)

| Priority | Model | Module | Current Access |
|----------|-------|--------|----------------|
| 1 | Site | site | via Server |
| 2 | Deployment | site | via Site |
| 3 | Certificate | site | via Site |
| 4 | Command | site | via Site |
| 5 | Queue | site | via Site |
| 6 | Redirect | site | via Site |
| 7 | Database | database | via Server |
| 8 | DatabaseUser | database | via Server |
| 9 | Backup | backup | via Server |
| 10 | BackupJob | backup | via Backup |
| 11 | DnsRecord | dns | via Domain |

### Models Requiring `team_id` NOT NULL Conversion (4 models)

| Model | Module | Current Type |
|-------|--------|--------------|
| SourceControl | git | `*string` (nullable) |
| Domain | dns | `*string` (nullable) |
| DomainProvider | dns | `*string` (nullable) |
| StorageProvider | backup | `*string` (nullable) |

---

## Task 1: Create Migration for Site team_id

**Files:**
- Create: `internal/database/migrations/0010_01_18_000000_add_team_id_to_sites_table.go`

**Step 1: Write the migration file**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000000_add_team_id_to_sites_table",
		Name:      "Add team_id to sites table",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 0, 0, time.UTC),
		Up:        addTeamIdToSitesTableUp,
		Down:      addTeamIdToSitesTableDown,
	})
}

func addTeamIdToSitesTableUp(db *gorm.DB) error {
	// Add team_id column
	if err := db.Exec(`ALTER TABLE sites ADD COLUMN team_id CHAR(26) NULL AFTER id`).Error; err != nil {
		return err
	}

	// Backfill team_id from servers table
	if err := db.Exec(`
		UPDATE sites s
		INNER JOIN servers srv ON s.server_id = srv.id
		SET s.team_id = srv.team_id
	`).Error; err != nil {
		return err
	}

	// Make column NOT NULL after backfill
	if err := db.Exec(`ALTER TABLE sites MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_sites_team_id ON sites(team_id)`).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(`
		ALTER TABLE sites
		ADD CONSTRAINT fk_sites_team_id
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToSitesTableDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE sites DROP FOREIGN KEY fk_sites_team_id`)
	db.Exec(`DROP INDEX idx_sites_team_id ON sites`)
	db.Exec(`ALTER TABLE sites DROP COLUMN team_id`)
	return nil
}
```

**Step 2: Run migration to verify it works**

```bash
make migrate-up
```

Expected: Migration runs successfully, sites table now has team_id column

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000000_add_team_id_to_sites_table.go
git commit -m "feat: add team_id column to sites table with backfill"
```

---

## Task 2: Update Site Model

**Files:**
- Modify: `internal/modules/site/models/site.go:21-25`

**Step 1: Add TeamID field to Site struct**

Add after line 24 (after `ServerID`):

```go
TeamID                       string           `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
```

The struct should now look like:

```go
type Site struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID                     string           `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	TeamID                       string           `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID                       string           `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	// ... rest of fields
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/modules/site/models/site.go
git commit -m "feat: add TeamID field to Site model"
```

---

## Task 3: Update Site Repository with Team Filtering

**Files:**
- Modify: `internal/modules/site/repositories/site_repository.go`

**Step 1: Add FindByIDAndTeam method**

Add after `FindByIDAndServer` method (around line 65):

```go
// FindByIDAndTeam finds a site by ID and team ID with custom error.
func (r *SiteRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Site, error) {
	var site models.Site
	err := r.DB.WithContext(ctx).
		First(&site, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return &site, nil
}

// FindByIDAndServerAndTeam finds a site by ID, server ID, and team ID.
func (r *SiteRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	var site models.Site
	err := r.DB.WithContext(ctx).
		First(&site, "id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return &site, nil
}

// FindAllByTeam finds all sites for a team
func (r *SiteRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&sites).Error
	return sites, err
}

// FindByServerAndTeam finds sites by server ID and team ID
func (r *SiteRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND team_id = ?", serverID, teamID).
		Order("created_at DESC").
		Find(&sites).Error
	return sites, err
}

// CountByTeam counts all sites for a team
func (r *SiteRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Site{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/modules/site/repositories/site_repository.go
git commit -m "feat: add team filtering methods to SiteRepository"
```

---

## Task 4: Update Site Service to Accept TeamID

**Files:**
- Modify: `internal/modules/site/services/site_service.go`

**Step 1: Update Create method to set TeamID**

Find the `Create` method and ensure it sets TeamID on the site. The teamID should be passed from the handler.

Update method signature and body to include teamID:

```go
// Create creates a new site
func (s *SiteService) Create(ctx context.Context, serverID, userID, teamID string, req *dto.CreateSiteRequest) (*models.Site, error) {
	// ... existing validation ...

	site := &models.Site{
		ServerID:  serverID,
		TeamID:    teamID,  // Add this line
		UserID:    userID,
		// ... rest of fields
	}

	// ... rest of method
}
```

**Step 2: Update FindByID method to filter by team**

```go
// FindByID finds a site by ID and validates team ownership
func (s *SiteService) FindByID(ctx context.Context, siteID, serverID, teamID string) (*models.Site, error) {
	return s.repos.Site().FindByIDAndServerAndTeam(ctx, siteID, serverID, teamID)
}
```

**Step 3: Verify compilation**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/modules/site/services/site_service.go
git commit -m "feat: update SiteService to require and validate team_id"
```

---

## Task 5: Update Site Handler to Pass TeamID

**Files:**
- Modify: `internal/modules/site/handlers/site_handler.go`

**Step 1: Update Create handler to extract and pass teamID**

```go
// Create creates a new site
func (h *SiteHandler) Create(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)  // Add this line

	// ... validation ...

	site, err := h.siteService.Create(c.Context(), serverID, userID, teamID, &req)  // Add teamID
	// ... rest
}
```

**Step 2: Update Show handler to extract and pass teamID**

```go
// Show returns a single site
func (h *SiteHandler) Show(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID := c.Locals("teamID").(string)  // Add this line

	site, err := h.siteService.FindByID(c.Context(), siteID, serverID, teamID)  // Add teamID
	// ... rest
}
```

**Step 3: Update all other handlers (List, Update, Delete, etc.) similarly**

Each handler should:
1. Extract `teamID := c.Locals("teamID").(string)`
2. Pass teamID to service methods

**Step 4: Verify compilation**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/modules/site/handlers/site_handler.go
git commit -m "feat: update Site handlers to enforce team isolation"
```

---

## Task 6: Create Migration for Deployment team_id

**Files:**
- Create: `internal/database/migrations/0010_01_18_000001_add_team_id_to_deployments_table.go`

**Step 1: Write the migration file**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000001_add_team_id_to_deployments_table",
		Name:      "Add team_id to deployments table",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 1, 0, time.UTC),
		Up:        addTeamIdToDeploymentsTableUp,
		Down:      addTeamIdToDeploymentsTableDown,
	})
}

func addTeamIdToDeploymentsTableUp(db *gorm.DB) error {
	// Add team_id column
	if err := db.Exec(`ALTER TABLE deployments ADD COLUMN team_id CHAR(26) NULL AFTER id`).Error; err != nil {
		return err
	}

	// Backfill team_id from sites table
	if err := db.Exec(`
		UPDATE deployments d
		INNER JOIN sites s ON d.site_id = s.id
		INNER JOIN servers srv ON s.server_id = srv.id
		SET d.team_id = srv.team_id
	`).Error; err != nil {
		return err
	}

	// Make column NOT NULL after backfill
	if err := db.Exec(`ALTER TABLE deployments MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_deployments_team_id ON deployments(team_id)`).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(`
		ALTER TABLE deployments
		ADD CONSTRAINT fk_deployments_team_id
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToDeploymentsTableDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE deployments DROP FOREIGN KEY fk_deployments_team_id`)
	db.Exec(`DROP INDEX idx_deployments_team_id ON deployments`)
	db.Exec(`ALTER TABLE deployments DROP COLUMN team_id`)
	return nil
}
```

**Step 2: Run migration**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000001_add_team_id_to_deployments_table.go
git commit -m "feat: add team_id column to deployments table with backfill"
```

---

## Task 7: Update Deployment Model

**Files:**
- Modify: `internal/modules/site/models/deployment.go:11-14`

**Step 1: Add TeamID field**

```go
type Deployment struct {
	basemodels.BaseModel
	SiteID     string                 `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	TeamID     string                 `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID     *string                `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	// ... rest
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/site/models/deployment.go
git commit -m "feat: add TeamID field to Deployment model"
```

---

## Task 8: Create Migration for Database team_id

**Files:**
- Create: `internal/database/migrations/0010_01_18_000002_add_team_id_to_databases_table.go`

**Step 1: Write the migration file**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000002_add_team_id_to_databases_table",
		Name:      "Add team_id to databases table",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 2, 0, time.UTC),
		Up:        addTeamIdToDatabasesTableUp,
		Down:      addTeamIdToDatabasesTableDown,
	})
}

func addTeamIdToDatabasesTableUp(db *gorm.DB) error {
	// Add team_id column
	if err := db.Exec(`ALTER TABLE databases ADD COLUMN team_id CHAR(26) NULL AFTER id`).Error; err != nil {
		return err
	}

	// Backfill team_id from servers table
	if err := db.Exec(`
		UPDATE databases d
		INNER JOIN servers srv ON d.server_id = srv.id
		SET d.team_id = srv.team_id
	`).Error; err != nil {
		return err
	}

	// Make column NOT NULL after backfill
	if err := db.Exec(`ALTER TABLE databases MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_databases_team_id ON databases(team_id)`).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(`
		ALTER TABLE databases
		ADD CONSTRAINT fk_databases_team_id
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToDatabasesTableDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE databases DROP FOREIGN KEY fk_databases_team_id`)
	db.Exec(`DROP INDEX idx_databases_team_id ON databases`)
	db.Exec(`ALTER TABLE databases DROP COLUMN team_id`)
	return nil
}
```

**Step 2: Run migration**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000002_add_team_id_to_databases_table.go
git commit -m "feat: add team_id column to databases table with backfill"
```

---

## Task 9: Update Database Model

**Files:**
- Modify: `internal/modules/database/models/database.go:8-16`

**Step 1: Add TeamID field**

```go
type Database struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	Name     string `gorm:"type:varchar(255);not null" json:"name"`

	// Relations
	Users []DatabaseUser `gorm:"many2many:database_database_user;" json:"users,omitempty"`
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/database/models/database.go
git commit -m "feat: add TeamID field to Database model"
```

---

## Task 10: Update Database Repository with Team Filtering

**Files:**
- Modify: `internal/modules/database/repositories/database_repository.go`

**Step 1: Add team filtering methods**

```go
// FindByIDAndTeam finds a database by ID and team ID
func (r *DatabaseRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Database, error) {
	var db models.Database
	err := r.DB.WithContext(ctx).
		First(&db, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}
		return nil, err
	}
	return &db, nil
}

// FindByIDAndServerAndTeam finds a database by ID, server ID, and team ID
func (r *DatabaseRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Database, error) {
	var db models.Database
	err := r.DB.WithContext(ctx).
		First(&db, "id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}
		return nil, err
	}
	return &db, nil
}

// FindByServerAndTeam finds all databases for a server and team
func (r *DatabaseRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Database, error) {
	var databases []models.Database
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND team_id = ?", serverID, teamID).
		Order("created_at DESC").
		Find(&databases).Error
	return databases, err
}

// CountByTeam counts all databases for a team
func (r *DatabaseRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Database{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/database/repositories/database_repository.go
git commit -m "feat: add team filtering methods to DatabaseRepository"
```

---

## Task 11: Update Database Handler to Pass TeamID

**Files:**
- Modify: `internal/modules/database/handlers/database_handler.go`

**Step 1: Update all handlers to extract and pass teamID**

```go
// ListDatabases returns all databases for a server
func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	teamID := c.Locals("teamID").(string)  // Add this

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)  // Pass teamID
	// ... rest
}

// CreateDatabase creates a new database
func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	teamID := c.Locals("teamID").(string)  // Add this

	// ... validation ...

	database, err := h.service.CreateDatabase(c.Context(), serverID, teamID, &req)  // Pass teamID
	// ... rest
}

// GetDatabase returns a single database
func (h *Handler) GetDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")
	teamID := c.Locals("teamID").(string)  // Add this

	database, err := h.service.GetDatabase(c.Context(), id, serverID, teamID)  // Pass teamID
	// ... rest
}

// DeleteDatabase deletes a database
func (h *Handler) DeleteDatabase(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	id := c.Params("id")
	teamID := c.Locals("teamID").(string)  // Add this

	err := h.service.DeleteDatabase(c.Context(), id, serverID, teamID)  // Pass teamID
	// ... rest
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/database/handlers/database_handler.go
git commit -m "feat: update Database handlers to enforce team isolation"
```

---

## Task 12: Create Migration for Backup team_id

**Files:**
- Create: `internal/database/migrations/0010_01_18_000003_add_team_id_to_backups_table.go`

**Step 1: Write the migration file**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000003_add_team_id_to_backups_table",
		Name:      "Add team_id to backups table",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 3, 0, time.UTC),
		Up:        addTeamIdToBackupsTableUp,
		Down:      addTeamIdToBackupsTableDown,
	})
}

func addTeamIdToBackupsTableUp(db *gorm.DB) error {
	// Add team_id column
	if err := db.Exec(`ALTER TABLE backups ADD COLUMN team_id CHAR(26) NULL AFTER id`).Error; err != nil {
		return err
	}

	// Backfill team_id from servers table
	if err := db.Exec(`
		UPDATE backups b
		INNER JOIN servers srv ON b.server_id = srv.id
		SET b.team_id = srv.team_id
	`).Error; err != nil {
		return err
	}

	// Make column NOT NULL after backfill
	if err := db.Exec(`ALTER TABLE backups MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_backups_team_id ON backups(team_id)`).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(`
		ALTER TABLE backups
		ADD CONSTRAINT fk_backups_team_id
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToBackupsTableDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE backups DROP FOREIGN KEY fk_backups_team_id`)
	db.Exec(`DROP INDEX idx_backups_team_id ON backups`)
	db.Exec(`ALTER TABLE backups DROP COLUMN team_id`)
	return nil
}
```

**Step 2: Run migration**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000003_add_team_id_to_backups_table.go
git commit -m "feat: add team_id column to backups table with backfill"
```

---

## Task 13: Update Backup Model

**Files:**
- Modify: `internal/modules/backup/models/backup.go:11-14`

**Step 1: Add TeamID field**

```go
type Backup struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID              string  `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	TeamID                string  `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID                *string `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	// ... rest
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/backup/models/backup.go
git commit -m "feat: add TeamID field to Backup model"
```

---

## Task 14: Create Migration for DatabaseUser team_id

**Files:**
- Create: `internal/database/migrations/0010_01_18_000004_add_team_id_to_database_users_table.go`

**Step 1: Write the migration file**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000004_add_team_id_to_database_users_table",
		Name:      "Add team_id to database_users table",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 4, 0, time.UTC),
		Up:        addTeamIdToDatabaseUsersTableUp,
		Down:      addTeamIdToDatabaseUsersTableDown,
	})
}

func addTeamIdToDatabaseUsersTableUp(db *gorm.DB) error {
	// Add team_id column
	if err := db.Exec(`ALTER TABLE database_users ADD COLUMN team_id CHAR(26) NULL AFTER id`).Error; err != nil {
		return err
	}

	// Backfill team_id from servers table
	if err := db.Exec(`
		UPDATE database_users du
		INNER JOIN servers srv ON du.server_id = srv.id
		SET du.team_id = srv.team_id
	`).Error; err != nil {
		return err
	}

	// Make column NOT NULL after backfill
	if err := db.Exec(`ALTER TABLE database_users MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_database_users_team_id ON database_users(team_id)`).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(`
		ALTER TABLE database_users
		ADD CONSTRAINT fk_database_users_team_id
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToDatabaseUsersTableDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE database_users DROP FOREIGN KEY fk_database_users_team_id`)
	db.Exec(`DROP INDEX idx_database_users_team_id ON database_users`)
	db.Exec(`ALTER TABLE database_users DROP COLUMN team_id`)
	return nil
}
```

**Step 2: Run migration**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000004_add_team_id_to_database_users_table.go
git commit -m "feat: add team_id column to database_users table with backfill"
```

---

## Task 15: Update DatabaseUser Model

**Files:**
- Modify: `internal/modules/database/models/database_user.go`

**Step 1: Add TeamID field**

```go
type DatabaseUser struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID string                    `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	TeamID   string                    `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	Username string                    `gorm:"type:varchar(255);not null" json:"username"`
	// ... rest
}
```

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/database/models/database_user.go
git commit -m "feat: add TeamID field to DatabaseUser model"
```

---

## Task 16: Create Migrations for Remaining Tables (Certificate, Command, Queue, Redirect, BackupJob, DnsRecord)

For each table, create a migration following the same pattern:

**Files to create:**
- `internal/database/migrations/0010_01_18_000005_add_team_id_to_certificates_table.go`
- `internal/database/migrations/0010_01_18_000006_add_team_id_to_commands_table.go`
- `internal/database/migrations/0010_01_18_000007_add_team_id_to_queues_table.go`
- `internal/database/migrations/0010_01_18_000008_add_team_id_to_redirects_table.go`
- `internal/database/migrations/0010_01_18_000009_add_team_id_to_backup_jobs_table.go`
- `internal/database/migrations/0010_01_18_000010_add_team_id_to_dns_records_table.go`

Each migration follows the same pattern:
1. Add nullable team_id column
2. Backfill from parent table (sites for site-related tables, backups for backup_jobs, domains for dns_records)
3. Make NOT NULL
4. Add index
5. Add foreign key

**Backfill queries by table:**

- **certificates**: `UPDATE certificates c INNER JOIN sites s ON c.site_id = s.id INNER JOIN servers srv ON s.server_id = srv.id SET c.team_id = srv.team_id`
- **commands**: `UPDATE commands c INNER JOIN sites s ON c.site_id = s.id INNER JOIN servers srv ON s.server_id = srv.id SET c.team_id = srv.team_id`
- **queues**: `UPDATE queues q INNER JOIN sites s ON q.site_id = s.id INNER JOIN servers srv ON s.server_id = srv.id SET q.team_id = srv.team_id`
- **redirects**: `UPDATE redirects r INNER JOIN sites s ON r.site_id = s.id INNER JOIN servers srv ON s.server_id = srv.id SET r.team_id = srv.team_id`
- **backup_jobs**: `UPDATE backup_jobs bj INNER JOIN backups b ON bj.backup_id = b.id INNER JOIN servers srv ON b.server_id = srv.id SET bj.team_id = srv.team_id`
- **dns_records**: `UPDATE dns_records dr INNER JOIN domains d ON dr.domain_id = d.id SET dr.team_id = d.team_id` (requires domains.team_id to be populated first)

**Step 1: Create all migration files following the pattern**

**Step 2: Run migrations**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_00000*.go
git commit -m "feat: add team_id columns to remaining tables"
```

---

## Task 17: Update Remaining Models (Certificate, Command, Queue, Redirect, BackupJob, DnsRecord)

**Files to modify:**
- `internal/modules/site/models/certificate.go`
- `internal/modules/site/models/command.go`
- `internal/modules/site/models/queue.go`
- `internal/modules/site/models/redirect.go`
- `internal/modules/backup/models/backup_job.go`
- `internal/modules/dns/models/dns_record.go`

Add to each model:

```go
TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
```

**Step 1: Update all model files**

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/*/models/*.go
git commit -m "feat: add TeamID field to remaining models"
```

---

## Task 18: Convert Nullable team_id to NOT NULL (SourceControl, Domain, DomainProvider, StorageProvider)

**Files:**
- Create: `internal/database/migrations/0010_01_18_000011_make_team_id_not_null.go`

**Step 1: Write migration to convert nullable columns**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000011_make_team_id_not_null",
		Name:      "Make team_id NOT NULL in source_controls, domains, domain_providers, storage_providers",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 11, 0, time.UTC),
		Up:        makeTeamIdNotNullUp,
		Down:      makeTeamIdNotNullDown,
	})
}

func makeTeamIdNotNullUp(db *gorm.DB) error {
	tables := []string{"source_controls", "domains", "domain_providers", "storage_providers"}

	for _, table := range tables {
		// Check if any NULL values exist and handle them
		// Option 1: Delete orphaned records (if acceptable)
		// Option 2: Assign to a default team (not recommended)
		// Here we'll make the column NOT NULL assuming data is already populated
		if err := db.Exec(`ALTER TABLE ` + table + ` MODIFY COLUMN team_id CHAR(26) NOT NULL`).Error; err != nil {
			return err
		}
	}

	return nil
}

func makeTeamIdNotNullDown(db *gorm.DB) error {
	tables := []string{"source_controls", "domains", "domain_providers", "storage_providers"}

	for _, table := range tables {
		if err := db.Exec(`ALTER TABLE ` + table + ` MODIFY COLUMN team_id CHAR(26) NULL`).Error; err != nil {
			return err
		}
	}

	return nil
}
```

**Step 2: Run migration**

```bash
make migrate-up
```

**Step 3: Commit**

```bash
git add internal/database/migrations/0010_01_18_000011_make_team_id_not_null.go
git commit -m "feat: make team_id NOT NULL in source_controls, domains, domain_providers, storage_providers"
```

---

## Task 19: Update Models to Use Non-Pointer TeamID

**Files to modify:**
- `internal/modules/git/models/source_control.go:14`
- `internal/modules/dns/models/domain.go`
- `internal/modules/dns/models/domain_provider.go`
- `internal/modules/backup/models/storage_provider.go`

**Step 1: Change TeamID from `*string` to `string`**

Before:
```go
TeamID *string `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
```

After:
```go
TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
```

**Step 2: Update any code that handles nullable TeamID**

Search for `TeamID` usages and update nil checks to use empty string checks if needed.

**Step 3: Verify compilation**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/modules/*/models/*.go
git commit -m "feat: change TeamID from pointer to value type"
```

---

## Task 20: Update All Services to Set TeamID on Create

For each service that creates resources, ensure TeamID is set:

**Files to check and update:**
- `internal/modules/site/services/site_service.go`
- `internal/modules/site/services/deployment_service.go`
- `internal/modules/site/services/certificate_service.go`
- `internal/modules/database/services/database_service.go`
- `internal/modules/backup/services/backup_service.go`
- `internal/modules/dns/services/domain_service.go`

**Pattern for each Create method:**

```go
func (s *Service) Create(ctx context.Context, teamID string, req *CreateRequest) (*Model, error) {
	model := &Model{
		TeamID: teamID,  // Always set team_id
		// ... other fields
	}
	return s.repo.Create(ctx, model)
}
```

**Step 1: Update all service Create methods**

**Step 2: Verify compilation**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/modules/*/services/*.go
git commit -m "feat: ensure all services set TeamID on resource creation"
```

---

## Task 21: Add Integration Tests for Team Isolation

**Files:**
- Create: `internal/modules/site/tests/team_isolation_test.go`

**Step 1: Write test to verify cross-team access is blocked**

```go
package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiteTeamIsolation(t *testing.T) {
	// Setup: Create two teams with servers and sites
	team1 := createTestTeam(t, "Team 1")
	team2 := createTestTeam(t, "Team 2")

	server1 := createTestServer(t, team1.ID)
	server2 := createTestServer(t, team2.ID)

	site1 := createTestSite(t, server1.ID, team1.ID)
	site2 := createTestSite(t, server2.ID, team2.ID)

	// Test: Team 1 should not access Team 2's site
	_, err := siteRepo.FindByIDAndTeam(context.Background(), site2.ID, team1.ID)
	assert.Error(t, err, "Team 1 should not access Team 2's site")

	// Test: Team 1 can access their own site
	found, err := siteRepo.FindByIDAndTeam(context.Background(), site1.ID, team1.ID)
	require.NoError(t, err)
	assert.Equal(t, site1.ID, found.ID)
}
```

**Step 2: Run tests**

```bash
go test ./internal/modules/site/tests/... -v
```

**Step 3: Commit**

```bash
git add internal/modules/site/tests/team_isolation_test.go
git commit -m "test: add team isolation tests for sites"
```

---

## Task 22: Final Verification and Cleanup

**Step 1: Run all tests**

```bash
make test
```

**Step 2: Run the application and verify API endpoints**

```bash
make run
```

Test these scenarios:
1. Create a site - should have team_id set
2. List sites - should only show current team's sites
3. Access site from different team - should return 403/404

**Step 3: Verify database schema**

```sql
DESCRIBE sites;
DESCRIBE deployments;
DESCRIBE databases;
DESCRIBE backups;
-- Verify team_id column exists and is NOT NULL
```

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: complete team isolation implementation"
```

---

## Summary

After completing all tasks:

1. **10 models** now have direct `team_id` columns (Site, Deployment, Certificate, Command, Queue, Redirect, Database, DatabaseUser, Backup, BackupJob, DnsRecord)

2. **4 models** converted from nullable to NOT NULL `team_id` (SourceControl, Domain, DomainProvider, StorageProvider)

3. All **repositories** have `FindByIDAndTeam()` and `FindByServerAndTeam()` methods

4. All **handlers** extract `teamID` from context and pass to services

5. All **services** set `TeamID` when creating resources

6. **Migrations** include backfill queries to populate existing data

7. **Tests** verify cross-team access is blocked

Each team now operates as a completely isolated workspace - creating a new team is like having an entirely new application.
