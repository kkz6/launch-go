package models

// SetDefaultStatus sets the status to the default value if it's currently empty.
// This is a simple helper for BeforeCreate hooks that need to set a default status.
//
// Unlike InitializeStatus (which requires implementing StatusInitializer interface),
// this function takes a direct pointer to the status field for simpler usage.
//
// Usage in BeforeCreate hooks:
//
//	func (s *Server) BeforeCreate(tx *gorm.DB) error {
//	    if err := s.BaseModel.BeforeCreate(tx); err != nil {
//	        return err
//	    }
//	    models.SetDefaultStatus(&s.Status, enums.ServerStatusNew)
//	    return nil
//	}
//
// The type constraint ~string allows this to work with any string-based enum type.
func SetDefaultStatus[T ~string](status *T, defaultStatus T) {
	if *status == "" {
		*status = defaultStatus
	}
}
