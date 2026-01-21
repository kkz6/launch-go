package dto

// EnumResponse represents a standardized enum value with its display label.
// This is the canonical response format for enum values in API responses.
//
// Example usage:
//
//	type ServerStatus string
//	const (
//	    StatusActive   ServerStatus = "active"
//	    StatusInactive ServerStatus = "inactive"
//	)
//	func (s ServerStatus) String() string { return string(s) }
//	func (s ServerStatus) Label() string { ... }
//
//	// Convert to response:
//	resp := dto.EnumToResponse(status)
type EnumResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Labeled is the interface that enums must implement to be convertible to EnumResponse.
// The String() method returns the raw enum value (e.g., "active").
// The Label() method returns the human-readable display text (e.g., "Active").
type Labeled interface {
	~string
	String() string
	Label() string
}

// EnumToResponse converts an enum value to an EnumResponse.
// The enum type must implement the Labeled interface.
//
// Example:
//
//	status := enums.ServerStatusActive
//	resp := dto.EnumToResponse(status)
//	// resp.Value = "active", resp.Label = "Active"
func EnumToResponse[E Labeled](e E) EnumResponse {
	return EnumResponse{
		Value: e.String(),
		Label: e.Label(),
	}
}

// EnumsToResponses converts a slice of enum values to EnumResponses.
//
// Example:
//
//	statuses := []enums.ServerStatus{enums.StatusActive, enums.StatusInactive}
//	responses := dto.EnumsToResponses(statuses)
func EnumsToResponses[E Labeled](enums []E) []EnumResponse {
	if enums == nil {
		return nil
	}
	result := make([]EnumResponse, len(enums))
	for i, e := range enums {
		result[i] = EnumToResponse(e)
	}
	return result
}

// EnumToResponseWithDescription extends EnumResponse with an optional description.
type EnumResponseWithDescription struct {
	Value       string  `json:"value"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"`
}

// LabeledWithDescription extends Labeled with a Description() method.
type LabeledWithDescription interface {
	Labeled
	Description() string
}

// EnumToResponseWithDesc converts an enum to a response including its description.
//
// Example:
//
//	resp := dto.EnumToResponseWithDesc(enums.ServerTypeApplication)
func EnumToResponseWithDesc[E LabeledWithDescription](e E) EnumResponseWithDescription {
	desc := e.Description()
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}
	return EnumResponseWithDescription{
		Value:       e.String(),
		Label:       e.Label(),
		Description: descPtr,
	}
}

// NewEnumResponse creates an EnumResponse from value and label strings.
// Use this when you need to create a response from raw strings rather than enum types.
//
// Example:
//
//	resp := dto.NewEnumResponse("active", "Active")
func NewEnumResponse(value, label string) EnumResponse {
	return EnumResponse{
		Value: value,
		Label: label,
	}
}

// EnumMapToResponses converts a map of enum values to labels into EnumResponses.
// This is useful when you have a pre-built map of values to labels.
//
// Example:
//
//	m := map[string]string{"active": "Active", "inactive": "Inactive"}
//	responses := dto.EnumMapToResponses(m)
func EnumMapToResponses(m map[string]string) []EnumResponse {
	if m == nil {
		return nil
	}
	result := make([]EnumResponse, 0, len(m))
	for value, label := range m {
		result = append(result, EnumResponse{
			Value: value,
			Label: label,
		})
	}
	return result
}
