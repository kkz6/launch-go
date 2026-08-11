package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	contractmocks "github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type transferTestResource struct {
	ID     uint   `gorm:"primaryKey"`
	TeamID string `gorm:"column:team_id;index"`
	Name   string
}

func (transferTestResource) TableName() string { return "transfer_test_resources" }

func newTeamServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.TeamInvitation{},
		&transferTestResource{},
	))
	return db
}

func seedTeamDeletion(t *testing.T, db *gorm.DB) (service *TeamService, source, destination *models.Team) {
	t.Helper()
	owner := &models.User{BaseModel: basemodels.BaseModel{ID: "owner"}, Name: "Owner", Email: "owner@example.com"}
	member := &models.User{BaseModel: basemodels.BaseModel{ID: "member"}, Name: "Member", Email: "member@example.com"}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(member).Error)

	source = &models.Team{BaseModel: basemodels.BaseModel{ID: "source"}, UserID: owner.ID, Name: "Source"}
	destination = &models.Team{BaseModel: basemodels.BaseModel{ID: "destination"}, UserID: owner.ID, Name: "Destination", PersonalTeam: true}
	require.NoError(t, db.Create(source).Error)
	require.NoError(t, db.Create(destination).Error)

	ownerRole, memberRole := "owner", "member"
	require.NoError(t, db.Create(&models.TeamMember{TeamID: source.ID, UserID: owner.ID, Role: &ownerRole}).Error)
	require.NoError(t, db.Create(&models.TeamMember{TeamID: source.ID, UserID: member.ID, Role: &memberRole}).Error)
	require.NoError(t, db.Create(&models.TeamMember{TeamID: destination.ID, UserID: owner.ID, Role: &ownerRole}).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id IN ?", []string{owner.ID, member.ID}).Update("current_team_id", source.ID).Error)
	require.NoError(t, db.Create(&models.TeamInvitation{TeamID: source.ID, Email: "pending@example.com", Role: &memberRole}).Error)
	require.NoError(t, db.Create(&transferTestResource{TeamID: source.ID, Name: "source-resource"}).Error)
	require.NoError(t, db.Create(&transferTestResource{TeamID: destination.ID, Name: "destination-resource"}).Error)

	require.NoError(t, db.Exec(`CREATE TABLE notification_preferences (
		id INTEGER PRIMARY KEY,
		team_id TEXT NOT NULL UNIQUE REFERENCES teams(id) ON DELETE CASCADE
	)`).Error)
	require.NoError(t, db.Exec("INSERT INTO notification_preferences (team_id) VALUES (?), (?)", source.ID, destination.ID).Error)
	require.NoError(t, db.Exec("CREATE TABLE impersonation_sessions (id INTEGER PRIMARY KEY, team_id TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO impersonation_sessions (team_id) VALUES (?)", source.ID).Error)

	return NewTeamService(repositories.NewRegistry(db), nil, nil), source, destination
}

func TestDeleteTeamTransfersResourcesAndSwitchesOwner(t *testing.T) {
	db := newTeamServiceTestDB(t)
	service, source, destination := seedTeamDeletion(t, db)
	cache := &recordingCache{}
	service.SetMembershipCache(launchcache.NewTeamMembershipCache(cache, db))

	result, err := service.DeleteTeam(context.Background(), "owner", source.ID, destination.ID)
	require.NoError(t, err)
	require.Equal(t, destination.ID, result.ID)

	var teamCount, memberCount, invitationCount, sourcePreferenceCount int64
	require.NoError(t, db.Model(&models.Team{}).Where("id = ?", source.ID).Count(&teamCount).Error)
	require.NoError(t, db.Model(&models.TeamMember{}).Where("team_id = ?", source.ID).Count(&memberCount).Error)
	require.NoError(t, db.Model(&models.TeamInvitation{}).Where("team_id = ?", source.ID).Count(&invitationCount).Error)
	require.NoError(t, db.Table("notification_preferences").Where("team_id = ?", source.ID).Count(&sourcePreferenceCount).Error)
	require.Zero(t, teamCount)
	require.Zero(t, memberCount)
	require.Zero(t, invitationCount)
	require.Zero(t, sourcePreferenceCount)

	var resources []transferTestResource
	require.NoError(t, db.Order("name").Find(&resources).Error)
	require.Len(t, resources, 2)
	require.Equal(t, destination.ID, resources[0].TeamID)
	require.Equal(t, destination.ID, resources[1].TeamID)

	var owner, member models.User
	require.NoError(t, db.First(&owner, "id = ?", "owner").Error)
	require.NoError(t, db.First(&member, "id = ?", "member").Error)
	require.Equal(t, destination.ID, *owner.CurrentTeamID)
	require.Nil(t, member.CurrentTeamID)

	var auditTeamID string
	require.NoError(t, db.Table("impersonation_sessions").Select("team_id").Scan(&auditTeamID).Error)
	require.Equal(t, source.ID, auditTeamID)
	require.ElementsMatch(t, []string{
		"team_membership:owner:source",
		"team_membership:owner:source",
		"team_membership:member:source",
	}, cache.deleted)
}

func TestDeleteTeamEmailsOwner(t *testing.T) {
	db := newTeamServiceTestDB(t)
	service, source, destination := seedTeamDeletion(t, db)
	sender := &recordingEmailSender{}
	logger := zerolog.Nop()
	service.emailSender = sender
	service.logger = &logger

	_, err := service.DeleteTeam(context.Background(), "owner", source.ID, destination.ID)
	require.NoError(t, err)
	require.Contains(t, sender.body, "Source")
	require.Contains(t, sender.body, "Destination")
}

func TestNewServiceWiresTeamDeletionEmail(t *testing.T) {
	logger := zerolog.Nop()
	sender := &recordingEmailSender{}
	cfg := &config.Config{
		App: config.AppConfig{Name: "Launch", URL: "https://launchctl.test"},
		Passkey: config.PasskeyConfig{
			RPName:   "Launch",
			RPID:     "launchctl.test",
			RPOrigin: "https://launchctl.test",
			Timeout:  60000,
		},
	}

	service, err := NewService(nil, cfg, &logger, sender, nil)
	require.NoError(t, err)
	require.Same(t, sender, service.Team.emailSender)
	require.Same(t, &logger, service.Team.logger)
}

type failingTeamDeletionSender struct{ err error }

func (s *failingTeamDeletionSender) Send(context.Context, string, string, string, bool) error {
	return s.err
}

func TestTeamDeletionEmailFailuresDoNotFailDeletion(t *testing.T) {
	service := NewTeamService(nil, nil, nil)
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}

	service.sendTeamDeletedEmail(context.Background(), owner, "Source", "Destination")
	service.emailSender = &recordingEmailSender{}
	service.sendTeamDeletedEmail(context.Background(), nil, "Source", "Destination")
	service.sendTeamDeletedEmail(context.Background(), &models.User{}, "Source", "Destination")

	wanted := errors.New("email unavailable")
	service.emailSender = &failingTeamDeletionSender{err: wanted}
	service.sendTeamDeletedEmail(context.Background(), owner, "Source", "Destination")

	logger := zerolog.Nop()
	service.emailSender = &failingTeamDeletionSender{err: wanted}
	service.logger = &logger
	service.sendTeamDeletedEmail(context.Background(), owner, "Source", "Destination")

	originalBuilder := buildTeamDeletedEmail
	buildTeamDeletedEmail = func(string, string) (string, string, error) {
		return "", "", wanted
	}
	t.Cleanup(func() { buildTeamDeletedEmail = originalBuilder })
	service.sendTeamDeletedEmail(context.Background(), owner, "Source", "Destination")
}

func TestDeleteTeamValidation(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		sourceID    string
		destination string
		want        func(error) bool
	}{
		{name: "source missing", userID: "owner", sourceID: "missing", destination: "destination", want: fiberutil.IsNotFound},
		{name: "not owner", userID: "member", sourceID: "source", destination: "destination", want: fiberutil.IsForbidden},
		{name: "same destination", userID: "owner", sourceID: "source", destination: "source", want: fiberutil.IsValidationError},
		{name: "destination missing", userID: "owner", sourceID: "source", destination: "missing", want: fiberutil.IsValidationError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := newTeamServiceTestDB(t)
			service, _, _ := seedTeamDeletion(t, db)
			result, err := service.DeleteTeam(context.Background(), test.userID, test.sourceID, test.destination)
			require.Nil(t, result)
			require.Error(t, err)
			require.True(t, test.want(err))
		})
	}
}

func TestDeleteTeamRejectsPersonalAndForeignDestination(t *testing.T) {
	db := newTeamServiceTestDB(t)
	service, source, destination := seedTeamDeletion(t, db)

	result, err := service.DeleteTeam(context.Background(), "owner", destination.ID, source.ID)
	require.Nil(t, result)
	require.EqualError(t, err, "cannot delete personal team")

	other := &models.User{BaseModel: basemodels.BaseModel{ID: "other"}, Name: "Other", Email: "other@example.com"}
	require.NoError(t, db.Create(other).Error)
	foreign := &models.Team{BaseModel: basemodels.BaseModel{ID: "foreign"}, UserID: other.ID, Name: "Foreign"}
	require.NoError(t, db.Create(foreign).Error)
	result, err = service.DeleteTeam(context.Background(), "owner", source.ID, foreign.ID)
	require.Nil(t, result)
	require.True(t, fiberutil.IsForbidden(err))
}

func TestDeleteTeamRollsBackResourceConflicts(t *testing.T) {
	db := newTeamServiceTestDB(t)
	service, source, destination := seedTeamDeletion(t, db)
	require.NoError(t, db.Exec("CREATE TABLE unique_team_resources (id INTEGER PRIMARY KEY, team_id TEXT, name TEXT, UNIQUE(team_id, name))").Error)
	require.NoError(t, db.Exec("INSERT INTO unique_team_resources (team_id, name) VALUES (?, 'same'), (?, 'same')", source.ID, destination.ID).Error)

	result, err := service.DeleteTeam(context.Background(), "owner", source.ID, destination.ID)
	require.Nil(t, result)
	require.Error(t, err)

	var sourceCount, sourceResourceCount int64
	require.NoError(t, db.Model(&models.Team{}).Where("id = ?", source.ID).Count(&sourceCount).Error)
	require.NoError(t, db.Model(&transferTestResource{}).Where("team_id = ?", source.ID).Count(&sourceResourceCount).Error)
	require.EqualValues(t, 1, sourceCount)
	require.EqualValues(t, 1, sourceResourceCount)
}

func TestTeamRequestNormalization(t *testing.T) {
	create := &dto.CreateTeamRequest{Name: "  New team  "}
	update := &dto.UpdateTeamRequest{Name: "  Renamed  "}
	create.Normalize()
	update.Normalize()
	require.Equal(t, "New team", create.Name)
	require.Equal(t, "Renamed", update.Name)
}

func TestDeleteTeamRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	wanted := fmt.Errorf("database unavailable")

	t.Run("source lookup", func(t *testing.T) {
		registry := contractmocks.NewRepositoryRegistry(t)
		teamRepo := contractmocks.NewTeamRepository(t)
		registry.EXPECT().Team().Return(teamRepo).Once()
		teamRepo.EXPECT().FindByID(ctx, "source").Return(nil, wanted).Once()
		result, err := NewTeamService(registry, nil, nil).DeleteTeam(ctx, "owner", "source", "destination")
		require.Nil(t, result)
		require.ErrorIs(t, err, wanted)
	})

	t.Run("destination lookup", func(t *testing.T) {
		registry := contractmocks.NewRepositoryRegistry(t)
		teamRepo := contractmocks.NewTeamRepository(t)
		registry.EXPECT().Team().Return(teamRepo).Twice()
		teamRepo.EXPECT().FindByID(ctx, "source").Return(&models.Team{UserID: "owner"}, nil).Once()
		teamRepo.EXPECT().FindByID(ctx, "destination").Return(nil, wanted).Once()
		result, err := NewTeamService(registry, nil, nil).DeleteTeam(ctx, "owner", "source", "destination")
		require.Nil(t, result)
		require.ErrorIs(t, err, wanted)
	})

	t.Run("member lookup", func(t *testing.T) {
		registry := contractmocks.NewRepositoryRegistry(t)
		teamRepo := contractmocks.NewTeamRepository(t)
		registry.EXPECT().Team().Return(teamRepo).Times(3)
		teamRepo.EXPECT().FindByID(ctx, "source").Return(&models.Team{UserID: "owner"}, nil).Once()
		teamRepo.EXPECT().FindByID(ctx, "destination").Return(&models.Team{UserID: "owner"}, nil).Once()
		teamRepo.EXPECT().GetMembers(ctx, "source").Return(nil, wanted).Once()
		result, err := NewTeamService(registry, nil, nil).DeleteTeam(ctx, "owner", "source", "destination")
		require.Nil(t, result)
		require.ErrorIs(t, err, wanted)
	})

	t.Run("owner lookup", func(t *testing.T) {
		registry := contractmocks.NewRepositoryRegistry(t)
		teamRepo := contractmocks.NewTeamRepository(t)
		userRepo := contractmocks.NewUserRepository(t)
		registry.EXPECT().Team().Return(teamRepo).Times(3)
		registry.EXPECT().User().Return(userRepo).Once()
		teamRepo.EXPECT().FindByID(ctx, "source").Return(&models.Team{UserID: "owner"}, nil).Once()
		teamRepo.EXPECT().FindByID(ctx, "destination").Return(&models.Team{UserID: "owner"}, nil).Once()
		teamRepo.EXPECT().GetMembers(ctx, "source").Return(nil, nil).Once()
		userRepo.EXPECT().FindByID(ctx, "owner").Return(nil, wanted).Once()
		result, err := NewTeamService(registry, nil, nil).DeleteTeam(ctx, "owner", "source", "destination")
		require.Nil(t, result)
		require.ErrorIs(t, err, wanted)
	})
}

func TestTransferResourcesTransactionErrors(t *testing.T) {
	openDB := func(t *testing.T) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
		require.NoError(t, err)
		return db
	}
	serviceFor := func(db *gorm.DB) *TeamService {
		return NewTeamService(repositories.NewRegistry(db), nil, nil)
	}

	t.Run("table discovery", func(t *testing.T) {
		db := openDB(t)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		err = serviceFor(db).transferResourcesAndDelete(context.Background(), "owner", "source", "destination")
		require.ErrorContains(t, err, "failed to discover team resources")
	})

	tests := []struct {
		name   string
		schema []string
	}{
		{name: "clear current team", schema: []string{"CREATE TABLE users (id TEXT PRIMARY KEY)"}},
		{name: "switch owner", schema: []string{"CREATE TABLE users (current_team_id TEXT, updated_at DATETIME)"}},
		{name: "delete members", schema: []string{"CREATE TABLE users (id TEXT PRIMARY KEY, current_team_id TEXT, updated_at DATETIME)"}},
		{name: "delete invitations", schema: []string{
			"CREATE TABLE users (id TEXT PRIMARY KEY, current_team_id TEXT, updated_at DATETIME)",
			"CREATE TABLE team_user (id INTEGER PRIMARY KEY, team_id TEXT)",
		}},
		{name: "delete team", schema: []string{
			"CREATE TABLE users (id TEXT PRIMARY KEY, current_team_id TEXT, updated_at DATETIME)",
			"CREATE TABLE team_user (id INTEGER PRIMARY KEY, team_id TEXT)",
			"CREATE TABLE team_invitations (id TEXT PRIMARY KEY, team_id TEXT)",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openDB(t)
			for _, statement := range test.schema {
				require.NoError(t, db.Exec(statement).Error)
			}
			err := serviceFor(db).transferResourcesAndDelete(context.Background(), "owner", "source", "destination")
			require.Error(t, err)
		})
	}
}

func TestFilterTransferableTeamTablesReturnsColumnErrors(t *testing.T) {
	wanted := errors.New("column lookup failed")
	tables, err := filterTransferableTeamTables([]string{"resources"}, func(string) ([]gorm.ColumnType, error) {
		return nil, wanted
	})
	require.Nil(t, tables)
	require.ErrorIs(t, err, wanted)
}

func TestInvalidateDeletedTeamMembershipsWithoutCache(t *testing.T) {
	NewTeamService(nil, nil, nil).invalidateDeletedTeamMemberships(context.Background(), "owner", "team", nil)
}
