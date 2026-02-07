package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000006_create_docker_domains_table",
		Name:      "Create docker domains table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 6, 0, time.UTC),
		Up:        createDockerDomainsTableUp,
		Down:      createDockerDomainsTableDown,
	})
}

type dockerDomainMigration struct {
	ID              string `gorm:"type:char(26);primaryKey"`
	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index"`

	Host            string `gorm:"type:varchar(255);not null"`
	Path            string `gorm:"type:varchar(255);not null;default:'/'"`
	ContainerPort   int    `gorm:"column:container_port;type:int;not null;default:80"`
	HTTPS           bool   `gorm:"column:https;not null;default:true"`
	CertificateType string `gorm:"column:certificate_type;type:varchar(50);not null;default:'letsencrypt'"`
	ForceSSL        bool   `gorm:"column:force_ssl;not null;default:true"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerDomainMigration) TableName() string { return "docker_domains" }

type dockerDomainWithServiceFK struct {
	DockerServiceID string                  `gorm:"column:docker_service_id"`
	DockerService   *dockerServiceMigration `gorm:"foreignKey:DockerServiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDomainWithServiceFK) TableName() string { return "docker_domains" }

func createDockerDomainsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dockerDomainMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerDomainWithServiceFK{}, "DockerService"); err != nil {
		return err
	}

	return db.Exec("CREATE UNIQUE INDEX idx_docker_domains_service_host_path ON docker_domains(docker_service_id, host, path)").Error
}

func createDockerDomainsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerDomainMigration{})
}
