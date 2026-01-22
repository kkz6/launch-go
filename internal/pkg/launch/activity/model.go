package activity

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

type ActivityLog struct {
	ID          uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	LogName     string         `json:"log_name" gorm:"index;size:255"`
	Description string         `json:"description" gorm:"type:text"`
	SubjectType *string        `json:"subject_type" gorm:"index;size:255"`
	SubjectID   *string        `json:"subject_id" gorm:"index;size:26"`
	CauserType  *string        `json:"causer_type" gorm:"index;size:255"`
	CauserID    *string        `json:"causer_id" gorm:"index;size:26"`
	Properties  dbtype.JSONMap `json:"properties" gorm:"type:json"`
	Event       *string        `json:"event" gorm:"size:255"`
	BatchUUID   *string        `json:"batch_uuid" gorm:"index;size:36"`
}

func (a *ActivityLog) TableName() string {
	return "activity_log"
}
