package migrations

import (
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"gorm.io/gorm"
	"time"
)

func init() {
	Register(Migration{ID: "0074_11_17_000000_github_webhook_intents", Name: "Persist GitHub delivery receipts and deployment intents", Timestamp: time.Date(2026, 11, 17, 0, 0, 4, 0, time.UTC), Up: func(db *gorm.DB) error {
		return db.AutoMigrate(&gitmodels.GitHubWebhookDelivery{}, &gitmodels.GitHubWebhookIntent{})
	}})
}
