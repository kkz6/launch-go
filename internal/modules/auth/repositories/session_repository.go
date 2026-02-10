package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// SessionRepository handles session database operations
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new SessionRepository instance
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	if session.ID == "" {
		session.ID = util.NewULID()
	}

	return r.db.WithContext(ctx).Create(session).Error
}

// FindByID finds a session by its ID
func (r *SessionRepository) FindByID(ctx context.Context, id string) (*models.Session, error) {
	var session models.Session
	err := r.db.WithContext(ctx).First(&session, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// GetByUser gets all sessions for a user, ordered by last activity
func (r *SessionRepository) GetByUser(ctx context.Context, userID string) ([]models.Session, error) {
	var sessions []models.Session
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_activity DESC").
		Find(&sessions).Error

	return sessions, err
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *SessionRepository) UpdateLastActivity(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&models.Session{}).
		Where("id = ?", id).
		Update("last_activity", time.Now().Unix()).Error
}

// Delete deletes a session by ID
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&models.Session{}).Error
}

// DeleteByUser deletes a specific session owned by a user
func (r *SessionRepository) DeleteByUser(ctx context.Context, id, userID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Session{})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// DeleteAllByUserExcept deletes all sessions for a user except the specified one
func (r *SessionRepository) DeleteAllByUserExcept(ctx context.Context, userID, exceptID string) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND id != ?", userID, exceptID).
		Delete(&models.Session{})

	return result.RowsAffected, result.Error
}

// DeleteAllByUser deletes all sessions for a user
func (r *SessionRepository) DeleteAllByUser(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.Session{}).Error
}

// Exists checks if a session exists
func (r *SessionRepository) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Session{}).
		Where("id = ?", id).
		Count(&count).Error

	return count > 0, err
}
