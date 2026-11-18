package registryaccess

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

const purpose = "ghcr_pull"

func Create(ctx context.Context, db *gorm.DB, deployment *models.Deployment, packageName, token, mintedAt, expiresAt string, snapshot *access.Snapshot) (string, error) {
	if deployment == nil || deployment.ID == "" || deployment.TeamID == "" || snapshot == nil || strings.TrimSpace(token) == "" || !strings.HasPrefix(packageName, "ghcr.io/") {
		return "", errors.New("GHCR pull authorization missing; re-sync the workflow")
	}
	minted, err := strconv.ParseInt(mintedAt, 10, 64)
	if err != nil {
		return "", errors.New("invalid GHCR token issue time")
	}
	expiry, err := strconv.ParseInt(expiresAt, 10, 64)
	if err != nil {
		return "", errors.New("GHCR token expiry missing; re-sync the workflow")
	}
	now := time.Now().UTC()
	issued := time.Unix(minted, 0)
	expires := time.Unix(expiry, 0)
	if issued.After(now.Add(30*time.Second)) || expires.After(issued.Add(time.Hour)) || !expires.After(now.Add(10*time.Second)) || !expires.After(issued) {
		return "", errors.New("GHCR pull credential is expired or has invalid lifetime")
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	lease := models.RegistryLease{TeamID: deployment.TeamID, DeploymentID: deployment.ID, TargetType: deployment.TargetType, TargetID: deployment.TargetID, Package: packageName, Purpose: purpose, Snapshot: string(data), Token: dbtype.EncryptedString(token), ExpiresAt: expires}
	// Expired secret material is removed opportunistically; references can never
	// be redeemed past ExpiresAt even when cleanup has not run yet.
	if err = db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&models.RegistryLease{}).Error; err != nil {
		return "", err
	}
	if err = db.WithContext(ctx).Create(&lease).Error; err != nil {
		return "", err
	}
	return lease.ID, nil
}

func Redeem(ctx context.Context, db *gorm.DB, id string, deployment *models.Deployment, packageName string, snapshot *access.Snapshot) (string, error) {
	if id == "" || deployment == nil || snapshot == nil {
		return "", errors.New("protected registry credential required")
	}
	var lease models.RegistryLease
	if err := db.WithContext(ctx).Where("id = ? AND team_id = ? AND deployment_id = ? AND target_type = ? AND target_id = ? AND package = ? AND purpose = ? AND expires_at > ?", id, deployment.TeamID, deployment.ID, deployment.TargetType, deployment.TargetID, packageName, purpose, time.Now().Add(5*time.Second)).First(&lease).Error; err != nil {
		return "", errors.New("registry credential expired or does not belong to this deployment")
	}
	var expected access.Snapshot
	if json.Unmarshal([]byte(lease.Snapshot), &expected) != nil || expected != *snapshot {
		return "", errors.New("registry credential authorization changed")
	}
	return lease.Token.String(), nil
}
func Delete(ctx context.Context, db *gorm.DB, id string) {
	if id != "" {
		db.WithContext(ctx).Where("id = ?", id).Delete(&models.RegistryLease{})
	}
}
