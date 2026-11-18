package migrations

import (
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
	"time"
)

func init() {
	Register(Migration{ID: "0072_11_17_000001_docker_registry_leases", Name: "Add protected deployment registry credentials", Timestamp: time.Date(2026, 11, 17, 0, 0, 1, 0, time.UTC), Up: func(db *gorm.DB) error { return db.AutoMigrate(&models.RegistryLease{}) }})
}
