package services

import (
	"context"
	"fmt"
	"sort"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	staffdto "github.com/kkz6/launch-go/internal/modules/staff/dto"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
)

// failuresCaveat documents the one class of failure this monitor cannot see:
// background asynq jobs fail in Redis, never touching the relational tables
// queried here.
const failuresCaveat = "Background job (asynq) failures are not captured here — they live in Redis only."

// failuresPerSourceLimit bounds how many rows we pull from each source so the
// merged, in-memory-sorted list stays bounded regardless of the requested page.
const failuresPerSourceLimit = 100

// detailMaxBytes caps the truncated task output included in a failure's Detail.
const detailMaxBytes = 2048

// Failures returns a unified, newest-first feed of operational failures across
// provisions, tasks and deployments. When kind is one of provision|task|
// deployment only that source is included; an empty kind merges all three. The
// total reflects the full merged list before offset/limit paging is applied.
func (s *Service) Failures(ctx context.Context, kind string, limit, offset int) (*staffdto.AdminFailuresResponse, int64, error) {
	merged := make([]staffdto.AdminFailure, 0)

	if kind == "" || kind == "provision" {
		servers, err := s.repos.FailedProvisions(ctx, failuresPerSourceLimit)
		if err != nil {
			return nil, 0, err
		}
		merged = append(merged, mapProvisions(servers)...)
	}

	if kind == "" || kind == "task" {
		tasks, err := s.repos.FailedTasks(ctx, failuresPerSourceLimit)
		if err != nil {
			return nil, 0, err
		}
		merged = append(merged, mapTasks(tasks)...)
	}

	if kind == "" || kind == "deployment" {
		rows, err := s.repos.FailedDeployments(ctx, failuresPerSourceLimit)
		if err != nil {
			return nil, 0, err
		}
		merged = append(merged, mapDeployments(rows)...)
	}

	sortFailuresByWhenDesc(merged)

	total := int64(len(merged))
	paged := pageFailures(merged, limit, offset)

	return &staffdto.AdminFailuresResponse{
		Failures: paged,
		Caveat:   failuresCaveat,
	}, total, nil
}

// mapProvisions maps failed servers to provision failures.
func mapProvisions(servers []servermodels.Server) []staffdto.AdminFailure {
	out := make([]staffdto.AdminFailure, 0, len(servers))
	for i := range servers {
		server := servers[i]

		errText := "provisioning failed"
		if server.ProvisionError != nil && *server.ProvisionError != "" {
			errText = *server.ProvisionError
		}

		out = append(out, staffdto.AdminFailure{
			Kind:     "provision",
			ID:       server.ID,
			Title:    server.Name,
			TeamID:   server.TeamID,
			ServerID: server.ID,
			When:     server.UpdatedAt,
			Error:    errText,
		})
	}

	return out
}

// mapTasks maps failed/timed-out tasks to task failures.
func mapTasks(tasks []servermodels.Task) []staffdto.AdminFailure {
	out := make([]staffdto.AdminFailure, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]

		title := task.Type
		if title == "" {
			title = task.Name
		}

		out = append(out, staffdto.AdminFailure{
			Kind:     "task",
			ID:       task.ID,
			Title:    title,
			ServerID: task.ServerID,
			When:     task.UpdatedAt,
			Error:    taskErrorText(task.ExitCode, task.Status),
			Detail:   truncate(task.Output.String(), detailMaxBytes),
		})
	}

	return out
}

// mapDeployments maps failed deployments (joined to their task) to deployment
// failures.
func mapDeployments(rows []repositories.FailedDeploymentRow) []staffdto.AdminFailure {
	out := make([]staffdto.AdminFailure, 0, len(rows))
	for i := range rows {
		row := rows[i]

		out = append(out, staffdto.AdminFailure{
			Kind:   "deployment",
			ID:     row.ID,
			Title:  "site " + row.SiteID,
			TeamID: row.TeamID,
			When:   row.UpdatedAt,
			Error:  taskErrorText(row.TaskExitCode, string(row.Status)),
			Detail: truncate(row.TaskOutput.String(), detailMaxBytes),
		})
	}

	return out
}

// taskErrorText prefers a non-zero exit code as the short error, falling back to
// the status string.
func taskErrorText(exitCode *int, status string) string {
	if exitCode != nil && *exitCode != 0 {
		return fmt.Sprintf("exit code %d", *exitCode)
	}

	return status
}

// sortFailuresByWhenDesc sorts failures newest-first. A nil When sorts last.
func sortFailuresByWhenDesc(failures []staffdto.AdminFailure) {
	sort.SliceStable(failures, func(i, j int) bool {
		left := failures[i].When
		right := failures[j].When

		if left == nil {
			return false
		}

		if right == nil {
			return true
		}

		return left.After(*right)
	})
}

// pageFailures applies offset/limit to an already-sorted slice. A non-positive
// limit returns everything from offset onward.
func pageFailures(failures []staffdto.AdminFailure, limit, offset int) []staffdto.AdminFailure {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(failures) {
		return []staffdto.AdminFailure{}
	}

	window := failures[offset:]

	if limit > 0 && limit < len(window) {
		window = window[:limit]
	}

	return window
}

// FailureLog returns the COMPLETE (untruncated) output for a single failure,
// looked up by its kind + id. The list/table path truncates Detail to keep the
// payload light; this powers the per-failure log viewer which needs the full
// text. Returns ErrFailureNotFound when the id does not resolve.
func (s *Service) FailureLog(ctx context.Context, kind, id string) (string, error) {
	switch kind {
	case "task":
		task, err := s.repos.FailedTaskByID(ctx, id)
		if err != nil {
			return "", err
		}
		if task == nil {
			return "", ErrFailureNotFound
		}
		return task.Output.String(), nil
	case "deployment":
		row, err := s.repos.FailedDeploymentByID(ctx, id)
		if err != nil {
			return "", err
		}
		if row == nil {
			return "", ErrFailureNotFound
		}
		return row.TaskOutput.String(), nil
	case "provision":
		server, err := s.repos.ServerByID(ctx, id)
		if err != nil {
			return "", err
		}
		if server == nil {
			return "", ErrFailureNotFound
		}
		if server.ProvisionError != nil {
			return *server.ProvisionError, nil
		}
		return "", nil
	default:
		return "", ErrFailureNotFound
	}
}

// truncate shortens s to at most maxBytes bytes, appending an ellipsis marker
// when content was dropped. It truncates on a byte boundary, then trims any
// partial trailing UTF-8 rune so the result stays valid UTF-8.
func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	cut := s[:maxBytes]
	for len(cut) > 0 && !utf8ValidLastRune(cut) {
		cut = cut[:len(cut)-1]
	}

	return cut + "…"
}

// utf8ValidLastRune reports whether s ends on a complete UTF-8 rune boundary.
func utf8ValidLastRune(s string) bool {
	if s == "" {
		return true
	}

	// Walk back over continuation bytes (0b10xxxxxx) to the lead byte.
	i := len(s) - 1
	for i >= 0 && s[i]&0xC0 == 0x80 {
		i--
	}

	if i < 0 {
		return false
	}

	lead := s[i]
	var size int
	switch {
	case lead&0x80 == 0x00:
		size = 1
	case lead&0xE0 == 0xC0:
		size = 2
	case lead&0xF0 == 0xE0:
		size = 3
	case lead&0xF8 == 0xF0:
		size = 4
	default:
		return false
	}

	return len(s)-i == size
}
