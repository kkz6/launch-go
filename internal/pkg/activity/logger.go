package activity

import (
	"context"
	"encoding/json"
	"reflect"

	"gorm.io/gorm"
)

type Subject interface {
	GetID() string
}

type Causer interface {
	GetID() string
}

type Logger struct {
	db          *gorm.DB
	logName     string
	description string
	subjectType *string
	subjectID   *string
	causerType  *string
	causerID    *string
	properties  map[string]any
	event       *string
	batchUUID   *string
	ctx         context.Context
}

func New(db *gorm.DB) *Logger {
	return &Logger{
		db:         db,
		logName:    "default",
		properties: make(map[string]any),
		ctx:        context.Background(),
	}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	l.ctx = ctx
	return l
}

func (l *Logger) UseLog(logName string) *Logger {
	l.logName = logName
	return l
}

func (l *Logger) On(subject Subject) *Logger {
	if subject == nil || reflect.ValueOf(subject).IsNil() {
		return l
	}

	subjectType := getTypeName(subject)
	subjectID := subject.GetID()

	l.subjectType = &subjectType
	l.subjectID = &subjectID
	return l
}

func (l *Logger) OnModel(subjectType, subjectID string) *Logger {
	l.subjectType = &subjectType
	l.subjectID = &subjectID
	return l
}

func (l *Logger) CausedBy(causer Causer) *Logger {
	if causer == nil || reflect.ValueOf(causer).IsNil() {
		return l
	}

	causerType := getTypeName(causer)
	causerID := causer.GetID()

	l.causerType = &causerType
	l.causerID = &causerID
	return l
}

func (l *Logger) CausedByUser(userID string) *Logger {
	causerType := "User"
	l.causerType = &causerType
	l.causerID = &userID
	return l
}

func (l *Logger) WithProperties(props map[string]any) *Logger {
	for k, v := range props {
		l.properties[k] = v
	}
	return l
}

func (l *Logger) WithProperty(key string, value any) *Logger {
	l.properties[key] = value
	return l
}

func (l *Logger) WithEvent(event string) *Logger {
	l.event = &event
	return l
}

func (l *Logger) InBatch(batchUUID string) *Logger {
	l.batchUUID = &batchUUID
	return l
}

func (l *Logger) Log(description string) (*ActivityLog, error) {
	l.description = description
	return l.save()
}

func (l *Logger) save() (*ActivityLog, error) {
	activity := &ActivityLog{
		LogName:     l.logName,
		Description: l.description,
		SubjectType: l.subjectType,
		SubjectID:   l.subjectID,
		CauserType:  l.causerType,
		CauserID:    l.causerID,
		Event:       l.event,
		BatchUUID:   l.batchUUID,
	}

	if len(l.properties) > 0 {
		propsJSON, err := json.Marshal(l.properties)
		if err != nil {
			return nil, err
		}
		activity.Properties = propsJSON
	}

	if err := l.db.WithContext(l.ctx).Create(activity).Error; err != nil {
		return nil, err
	}

	return activity, nil
}

func getTypeName(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
