package repositories

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestGetUserTeamsIsDistinctAndDeterministic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}))

	user := &models.User{BaseModel: basemodels.BaseModel{ID: "user"}, Name: "User", Email: "user@example.com"}
	other := &models.User{BaseModel: basemodels.BaseModel{ID: "other"}, Name: "Other", Email: "other@example.com"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(other).Error)

	personal := &models.Team{BaseModel: basemodels.BaseModel{ID: "personal"}, UserID: user.ID, Name: "Zeta", PersonalTeam: true}
	owned := &models.Team{BaseModel: basemodels.BaseModel{ID: "owned"}, UserID: user.ID, Name: "beta"}
	joined := &models.Team{BaseModel: basemodels.BaseModel{ID: "joined"}, UserID: other.ID, Name: "Alpha"}
	require.NoError(t, db.Create([]*models.Team{personal, owned, joined}).Error)
	ownerRole, memberRole := "owner", "member"
	require.NoError(t, db.Create(&models.TeamMember{TeamID: personal.ID, UserID: user.ID, Role: &ownerRole}).Error)
	require.NoError(t, db.Create(&models.TeamMember{TeamID: owned.ID, UserID: user.ID, Role: &ownerRole}).Error)
	require.NoError(t, db.Create(&models.TeamMember{TeamID: joined.ID, UserID: user.ID, Role: &memberRole}).Error)

	teams, err := NewTeamRepository(db).GetUserTeams(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, teams, 3)
	require.Equal(t, []string{"personal", "joined", "owned"}, []string{teams[0].ID, teams[1].ID, teams[2].ID})
}

func TestGetUserTeamsReturnsDatabaseError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	teams, err := NewTeamRepository(db).GetUserTeams(context.Background(), "user")
	require.Error(t, err)
	require.Empty(t, teams)
}

func TestGetUserTeamsQueryIsPostgresSafe(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "postgres://launch:launch@localhost:5432/launch?sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	require.NoError(t, err)

	var teams []models.Team
	query := NewTeamRepository(db).userTeamsQuery(context.Background(), "user").
		Order("teams.personal_team DESC").
		Order("LOWER(teams.name) ASC").
		Order("teams.id ASC").
		Find(&teams)
	require.NoError(t, query.Error)

	sql := strings.ToUpper(query.Statement.SQL.String())
	require.NotContains(t, sql, "SELECT DISTINCT")
	require.Contains(t, sql, "EXISTS")
	require.Contains(t, sql, "LOWER(TEAMS.NAME)")
}
