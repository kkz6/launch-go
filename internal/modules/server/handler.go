package server

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Server CRUD handlers

func (h *Handler) List(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "15"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 15
	}

	servers, total, err := h.service.ListServersPaginated(c.Context(), teamID, page, perPage)
	if err != nil {
		return response.InternalError(c, "Failed to fetch servers")
	}

	result := make([]ServerResponse, len(servers))
	for i, server := range servers {
		result[i] = ToServerResponse(&server)
	}

	return response.OK(c, "Servers retrieved", fiber.Map{
		"servers": result,
		"total":   total,
		"page":    page,
		"perPage": perPage,
	})
}

func (h *Handler) Create(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req CreateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	server, err := h.service.CreateServer(c.Context(), teamID, userID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidProvider) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid server provider")
		}
		if errors.Is(err, ErrInvalidServerType) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid server type")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Server created", ToServerResponse(server))
}

func (h *Handler) Show(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	server, err := h.service.GetServerWithRelations(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch server")
	}

	return response.OK(c, "Server retrieved", ToServerResponse(server))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req UpdateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	server, err := h.service.UpdateServer(c.Context(), id, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server updated", ToServerResponse(server))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.DeleteServer(c.Context(), id, teamID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// Server actions

func (h *Handler) Reboot(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.RebootServer(c.Context(), id, teamID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrServerNotProvisioned) {
			return response.Error(c, fiber.StatusBadRequest, "Server is not provisioned")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server reboot initiated", nil)
}

func (h *Handler) Connect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.ConnectServer(c.Context(), id, teamID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server connection successful", nil)
}

func (h *Handler) Archive(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.ArchiveServer(c.Context(), id, teamID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server archived", nil)
}

func (h *Handler) Unarchive(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.UnarchiveServer(c.Context(), id, teamID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Server unarchived", nil)
}

func (h *Handler) ShowPage(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	data, err := h.service.GetShowPageData(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch server data")
	}

	return response.OK(c, "Server page data retrieved", data)
}

// Services handlers

func (h *Handler) ListServices(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	services, err := h.service.ListServices(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch services")
	}

	result := make([]ServiceResponse, len(services))
	for i, svc := range services {
		result[i] = ToServiceResponse(&svc)
	}

	return response.OK(c, "Services retrieved", result)
}

func (h *Handler) InstallService(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateServiceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	svc, err := h.service.InstallService(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrInvalidSoftware) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid software")
		}
		if errors.Is(err, ErrServiceAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "Service already installed")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Service installation initiated", ToServiceResponse(svc))
}

func (h *Handler) ServiceOperation(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	serviceID := c.Params("serviceId")

	var req ServiceOperationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	operation, err := ParseServiceOption(req.Operation)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid operation")
	}

	if err := h.service.HandleServiceOperation(c.Context(), serverID, teamID, serviceID, operation); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrServiceNotFound) {
			return response.NotFound(c, "Service not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Service operation initiated", nil)
}

// Firewall rules handlers

func (h *Handler) ListFirewallRules(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	rules, err := h.service.ListFirewallRules(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch firewall rules")
	}

	result := make([]FirewallRuleResponse, len(rules))
	for i, rule := range rules {
		result[i] = ToFirewallRuleResponse(&rule)
	}

	return response.OK(c, "Firewall rules retrieved", result)
}

func (h *Handler) CreateFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateFirewallRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	rule, err := h.service.CreateFirewallRule(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Firewall rule created", ToFirewallRuleResponse(rule))
}

func (h *Handler) UpdateFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	var req UpdateFirewallRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	rule, err := h.service.UpdateFirewallRule(c.Context(), serverID, teamID, ruleID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrFirewallRuleNotFound) {
			return response.NotFound(c, "Firewall rule not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Firewall rule updated", ToFirewallRuleResponse(rule))
}

func (h *Handler) DeleteFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	if err := h.service.DeleteFirewallRule(c.Context(), serverID, teamID, ruleID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrFirewallRuleNotFound) {
			return response.NotFound(c, "Firewall rule not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// Cron handlers

func (h *Handler) ListCrons(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	crons, err := h.service.ListCrons(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch cron jobs")
	}

	result := make([]CronResponse, len(crons))
	for i, cron := range crons {
		result[i] = ToCronResponse(&cron)
	}

	return response.OK(c, "Cron jobs retrieved", result)
}

func (h *Handler) CreateCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateCronRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	cron, err := h.service.CreateCron(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Cron job created", ToCronResponse(cron))
}

func (h *Handler) UpdateCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	cronID := c.Params("cronId")

	var req UpdateCronRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	cron, err := h.service.UpdateCron(c.Context(), serverID, teamID, cronID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrCronNotFound) {
			return response.NotFound(c, "Cron job not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Cron job updated", ToCronResponse(cron))
}

func (h *Handler) DeleteCron(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	cronID := c.Params("cronId")

	if err := h.service.DeleteCron(c.Context(), serverID, teamID, cronID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrCronNotFound) {
			return response.NotFound(c, "Cron job not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// Daemon handlers

func (h *Handler) ListDaemons(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	daemons, err := h.service.ListDaemons(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch daemons")
	}

	result := make([]DaemonResponse, len(daemons))
	for i, daemon := range daemons {
		result[i] = ToDaemonResponse(&daemon)
	}

	return response.OK(c, "Daemons retrieved", result)
}

func (h *Handler) CreateDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateDaemonRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	daemon, err := h.service.CreateDaemon(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Daemon created", ToDaemonResponse(daemon))
}

func (h *Handler) UpdateDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	var req UpdateDaemonRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	daemon, err := h.service.UpdateDaemon(c.Context(), serverID, teamID, daemonID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrDaemonNotFound) {
			return response.NotFound(c, "Daemon not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Daemon updated", ToDaemonResponse(daemon))
}

func (h *Handler) DeleteDaemon(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	daemonID := c.Params("daemonId")

	if err := h.service.DeleteDaemon(c.Context(), serverID, teamID, daemonID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrDaemonNotFound) {
			return response.NotFound(c, "Daemon not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// SSH Key handlers

func (h *Handler) ListSshKeys(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	keys, err := h.service.ListSshKeys(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]SshKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = ToSshKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

func (h *Handler) ListServerSshKeys(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	keys, err := h.service.ListServerSshKeys(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]SshKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = ToSshKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

func (h *Handler) CreateSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req CreateSshKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	key, err := h.service.CreateSshKey(c.Context(), teamID, userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "SSH key created", ToSshKeyResponse(key))
}

func (h *Handler) AttachSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req AttachSshKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.AttachSshKey(c.Context(), serverID, teamID, req.SshKeyID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "SSH key attached", nil)
}

func (h *Handler) DetachSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DetachSshKey(c.Context(), serverID, teamID, sshKeyID); err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		if errors.Is(err, ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

func (h *Handler) DeleteSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DeleteSshKey(c.Context(), teamID, sshKeyID); err != nil {
		if errors.Is(err, ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// Database handlers

func (h *Handler) ListDatabases(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	databases, err := h.service.ListDatabases(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch databases")
	}

	return response.OK(c, "Databases retrieved", databases)
}

func (h *Handler) CreateDatabase(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req CreateDatabaseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	db, err := h.service.CreateDatabase(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Database created", db)
}

// Task handlers

func (h *Handler) ListTasks(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	if limit < 1 || limit > 100 {
		limit = 50
	}

	tasks, err := h.service.ListTasks(c.Context(), serverID, teamID, limit)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch tasks")
	}

	result := make([]TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = ToTaskResponse(&task)
	}

	return response.OK(c, "Tasks retrieved", result)
}

func (h *Handler) GetLatestTask(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	task, err := h.service.GetLatestTask(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch task")
	}

	if task == nil {
		return response.NotFound(c, "No tasks found")
	}

	return response.OK(c, "Latest task retrieved", ToTaskResponse(task))
}

// Metric handlers

func (h *Handler) GetLatestMetric(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	metric, err := h.service.GetLatestMetric(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch metric")
	}

	if metric == nil {
		return response.NotFound(c, "No metrics found")
	}

	return response.OK(c, "Latest metric retrieved", ToMetricResponse(metric))
}

func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	limit, _ := strconv.Atoi(c.Query("limit", "100"))

	if limit < 1 || limit > 1000 {
		limit = 100
	}

	metrics, err := h.service.GetMetrics(c.Context(), serverID, teamID, nil, nil, limit)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}
		return response.InternalError(c, "Failed to fetch metrics")
	}

	result := make([]MetricResponse, len(metrics))
	for i, metric := range metrics {
		result[i] = ToMetricResponse(&metric)
	}

	return response.OK(c, "Metrics retrieved", result)
}
