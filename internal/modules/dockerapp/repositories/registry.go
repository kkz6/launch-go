package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
)

// Registry holds all dockerapp repositories.
type Registry struct {
	db     *gorm.DB
	app    *AppRepository
	envVar *EnvVarRepository
	port   *PortRepository
	volume *VolumeRepository
	domain *DomainRepository
}

// NewRegistry creates the registry.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:     db,
		app:    NewAppRepository(db),
		envVar: NewEnvVarRepository(db),
		port:   NewPortRepository(db),
		volume: NewVolumeRepository(db),
		domain: NewDomainRepository(db),
	}
}

// App returns the app repository.
func (r *Registry) App() contracts.AppRepository { return r.app }

// EnvVar returns the env-var repository.
func (r *Registry) EnvVar() contracts.EnvVarRepository { return r.envVar }

// Port returns the port repository.
func (r *Registry) Port() contracts.PortRepository { return r.port }

// Volume returns the volume repository.
func (r *Registry) Volume() contracts.VolumeRepository { return r.volume }

// Domain returns the domain repository.
func (r *Registry) Domain() contracts.DomainRepository { return r.domain }

// DB returns the underlying database connection.
func (r *Registry) DB() *gorm.DB { return r.db }
