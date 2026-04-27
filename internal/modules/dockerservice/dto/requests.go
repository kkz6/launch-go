package dto

import "github.com/kkz6/launch-go/internal/modules/dockerservice/types"

// InstallDockerServiceRequest is the body of POST install.
type InstallDockerServiceRequest struct {
	Kind         types.Kind `json:"kind" validate:"required"`
	Image        string     `json:"image" validate:"omitempty,max=255"`
	Username     string     `json:"username" validate:"omitempty,min=1,max=32"`
	Password     string     `json:"password" validate:"omitempty,min=8,max=128"`
	DatabaseName string     `json:"database_name" validate:"omitempty,min=1,max=64"`
}

// UninstallDockerServiceRequest is the body of DELETE.
type UninstallDockerServiceRequest struct {
	RemoveData bool `json:"remove_data"`
}
