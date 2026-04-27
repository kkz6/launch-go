package dto

import "github.com/kkz6/launch-go/internal/modules/managedservice/types"

// InstallManagedServiceRequest is the body of POST install.
type InstallManagedServiceRequest struct {
	Kind         types.Kind `json:"kind" validate:"required"`
	Image        string     `json:"image" validate:"omitempty,max=255"`
	Username     string     `json:"username" validate:"omitempty,min=1,max=32"`
	Password     string     `json:"password" validate:"omitempty,min=8,max=128"`
	DatabaseName string     `json:"database_name" validate:"omitempty,min=1,max=64"`
}

// UninstallManagedServiceRequest is the body of DELETE.
type UninstallManagedServiceRequest struct {
	RemoveData bool `json:"remove_data"`
}
