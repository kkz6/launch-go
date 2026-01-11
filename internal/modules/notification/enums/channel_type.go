package enums

import (
	"database/sql/driver"
	"fmt"
)

// ChannelType represents a notification channel type
type ChannelType string

const (
	ChannelTypeEmail    ChannelType = "email"
	ChannelTypeSlack    ChannelType = "slack"
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeTelegram ChannelType = "telegram"
)

// AllChannelTypes returns all available channel types
func AllChannelTypes() []ChannelType {
	return []ChannelType{
		ChannelTypeEmail,
		ChannelTypeSlack,
		ChannelTypeDiscord,
		ChannelTypeTelegram,
	}
}

// String returns the string value of the channel type
func (c ChannelType) String() string {
	return string(c)
}

// Label returns a human-readable label for the channel type
func (c ChannelType) Label() string {
	switch c {
	case ChannelTypeEmail:
		return "Email"
	case ChannelTypeSlack:
		return "Slack"
	case ChannelTypeDiscord:
		return "Discord"
	case ChannelTypeTelegram:
		return "Telegram"
	default:
		return string(c)
	}
}

// IsValid checks if the channel type is valid
func (c ChannelType) IsValid() bool {
	switch c {
	case ChannelTypeEmail, ChannelTypeSlack, ChannelTypeDiscord, ChannelTypeTelegram:
		return true
	}

	return false
}

// Scan implements the sql.Scanner interface
func (c *ChannelType) Scan(value interface{}) error {
	if value == nil {
		*c = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*c = ChannelType(v)
	case []byte:
		*c = ChannelType(string(v))
	default:
		return fmt.Errorf("cannot scan type %T into ChannelType", value)
	}

	return nil
}

// Value implements the driver.Valuer interface
func (c ChannelType) Value() (driver.Value, error) {
	return string(c), nil
}

// ParseChannelType parses a string into a ChannelType
func ParseChannelType(s string) (ChannelType, error) {
	ct := ChannelType(s)
	if !ct.IsValid() {
		return "", fmt.Errorf("invalid channel type: %s", s)
	}

	return ct, nil
}
