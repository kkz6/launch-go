package enumtypes

// StringEnum is the interface that all string-based enums should implement.
type StringEnum interface {
	~string
	String() string
	IsValid() bool
}

// LabeledEnum extends StringEnum with a human-readable label.
type LabeledEnum interface {
	StringEnum
	Label() string
}
