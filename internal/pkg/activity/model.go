package activity

import (
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

type ActivityLog struct {
	ID            string          `json:"id" gorm:"primaryKey;size:26"`
	LogName       string          `json:"log_name" gorm:"index;size:255"`
	Description   string          `json:"description" gorm:"type:text"`
	SubjectType   *string         `json:"subject_type" gorm:"index;size:255"`
	SubjectID     *string         `json:"subject_id" gorm:"index;size:26"`
	CauserType    *string         `json:"causer_type" gorm:"index;size:255"`
	CauserID      *string         `json:"causer_id" gorm:"index;size:26"`
	Properties    json.RawMessage `json:"properties" gorm:"type:json"`
	Event         *string         `json:"event" gorm:"size:255"`
	BatchUUID     *string         `json:"batch_uuid" gorm:"index;size:36"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (a *ActivityLog) TableName() string {
	return "activity_log"
}

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = ulid.Make().String()
	}
	return nil
}

func (a *ActivityLog) GetProperties() map[string]any {
	if a.Properties == nil {
		return make(map[string]any)
	}
	var props map[string]any
	if err := json.Unmarshal(a.Properties, &props); err != nil {
		return make(map[string]any)
	}
	return props
}

func (a *ActivityLog) SetProperties(props map[string]any) error {
	data, err := json.Marshal(props)
	if err != nil {
		return err
	}
	a.Properties = data
	return nil
}
