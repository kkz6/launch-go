package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

type Service struct {
	repo   *Repository
	queue  *queue.Client
	ws     *websocket.Hub
	logger *zerolog.Logger
}

func NewService(repo *Repository, queue *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		queue:  queue,
		ws:     ws,
		logger: logger,
	}
}

func (s *Service) List(ctx context.Context, teamID string) ([]Server, error) {
	return s.repo.FindAllByTeam(ctx, teamID)
}

func (s *Service) Create(ctx context.Context, teamID string, req *CreateServerRequest) (*Server, error) {
	// Generate SSH key pair
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return nil, err
	}

	server := &Server{
		TeamID:       teamID,
		Name:         req.Name,
		Provider:     ServerProvider(req.Provider),
		Region:       req.Region,
		Size:         req.Size,
		Status:       ServerStatusPending,
		SSHPort:      22,
		SSHUser:      "root",
		PrivateKey:   privateKey,
		PublicKey:    publicKey,
		PHPVersion:   req.PHPVersion,
		DatabaseType: &req.DatabaseType,
	}

	if req.SSHPort > 0 {
		server.SSHPort = req.SSHPort
	}
	if req.SSHUser != "" {
		server.SSHUser = req.SSHUser
	}
	if req.PHPVersion == "" {
		server.PHPVersion = "8.3"
	}

	// For custom servers, use provided key
	if req.Provider == "custom" {
		server.IPAddress = &req.IPAddress
		server.PrivateKey = req.PrivateKey
		server.PublicKey = ""
	}

	if err := s.repo.Create(ctx, server); err != nil {
		return nil, err
	}

	// Dispatch provisioning job
	task, err := jobs.NewProvisionTask(server.ID, teamID)
	if err != nil {
		return nil, err
	}

	if _, err := s.queue.EnqueueCritical(task); err != nil {
		s.logger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to enqueue provision job")
	}

	return server, nil
}

func (s *Service) FindByID(ctx context.Context, id, teamID string) (*Server, error) {
	return s.repo.FindByIDAndTeam(ctx, id, teamID)
}

func (s *Service) Update(ctx context.Context, id, teamID string, req *UpdateServerRequest) (*Server, error) {
	server, err := s.repo.FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, err
	}

	server.Name = req.Name

	if req.PHPVersion != "" && req.PHPVersion != server.PHPVersion {
		// Dispatch PHP version change job
		task, err := jobs.NewInstallPHPTask(server.ID, req.PHPVersion)
		if err == nil {
			s.queue.EnqueueDefault(task)
		}
		server.PHPVersion = req.PHPVersion
	}

	if err := s.repo.Update(ctx, server); err != nil {
		return nil, err
	}

	return server, nil
}

func (s *Service) Delete(ctx context.Context, id, teamID string) error {
	server, err := s.repo.FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// Update status to deleting
	s.repo.UpdateStatus(ctx, id, ServerStatusDeleting)

	// TODO: Dispatch deletion job to cloud provider

	return s.repo.Delete(ctx, server.ID)
}

func (s *Service) Reboot(ctx context.Context, id, teamID string) error {
	server, err := s.repo.FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	task, err := jobs.NewRebootTask(server.ID)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

// Database operations
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *CreateDatabaseRequest) (*Database, error) {
	// Verify server belongs to team
	if _, err := s.repo.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	db := &Database{
		ServerID: serverID,
		Name:     req.Name,
		Type:     req.Type,
	}

	if err := s.repo.CreateDatabase(ctx, db); err != nil {
		return nil, err
	}

	// TODO: Dispatch job to create database on server

	return db, nil
}

func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]Database, error) {
	if _, err := s.repo.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	return s.repo.FindDatabasesByServer(ctx, serverID)
}

// Broadcast helpers
func (s *Service) BroadcastStatus(serverID string, status ServerStatus, message string) {
	s.ws.BroadcastToServer(serverID, "server.status", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(publicKey))

	return string(privateKeyPEM), publicKeyStr, nil
}
