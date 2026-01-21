package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTimestampResponse(t *testing.T) {
	t.Run("formats both timestamps", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

		resp := NewTimestampResponse(&created, &updated)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-01-16T12:00:00Z", resp.UpdatedAt)
	})

	t.Run("handles nil timestamps", func(t *testing.T) {
		resp := NewTimestampResponse(nil, nil)

		assert.Equal(t, "", resp.CreatedAt)
		assert.Equal(t, "", resp.UpdatedAt)
	})

	t.Run("handles mixed nil timestamps", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

		resp := NewTimestampResponse(&created, nil)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Equal(t, "", resp.UpdatedAt)
	})
}

func TestNewTimestampResponseFromValues(t *testing.T) {
	t.Run("formats time values", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

		resp := NewTimestampResponseFromValues(created, updated)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-01-16T12:00:00Z", resp.UpdatedAt)
	})
}

func TestNewTimestampPtrResponse(t *testing.T) {
	t.Run("formats as pointer strings", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

		resp := NewTimestampPtrResponse(&created, &updated)

		assert.NotNil(t, resp.CreatedAt)
		assert.NotNil(t, resp.UpdatedAt)
		assert.Equal(t, "2024-01-15T10:00:00Z", *resp.CreatedAt)
		assert.Equal(t, "2024-01-16T12:00:00Z", *resp.UpdatedAt)
	})

	t.Run("returns nil for nil inputs", func(t *testing.T) {
		resp := NewTimestampPtrResponse(nil, nil)

		assert.Nil(t, resp.CreatedAt)
		assert.Nil(t, resp.UpdatedAt)
	})
}

func TestNewAuditTimestampResponse(t *testing.T) {
	t.Run("includes deleted_at", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)
		deleted := time.Date(2024, 1, 17, 14, 0, 0, 0, time.UTC)

		resp := NewAuditTimestampResponse(&created, &updated, &deleted)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-01-16T12:00:00Z", resp.UpdatedAt)
		assert.NotNil(t, resp.DeletedAt)
		assert.Equal(t, "2024-01-17T14:00:00Z", *resp.DeletedAt)
	})

	t.Run("deleted_at nil when not deleted", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

		resp := NewAuditTimestampResponse(&created, &updated, nil)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Nil(t, resp.DeletedAt)
	})
}

func TestNewInstallableTimestampResponse(t *testing.T) {
	t.Run("includes installed_at", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)
		installed := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

		resp := NewInstallableTimestampResponse(&created, &updated, &installed)

		assert.Equal(t, "2024-01-15T10:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-01-16T12:00:00Z", resp.UpdatedAt)
		assert.NotNil(t, resp.InstalledAt)
		assert.Equal(t, "2024-01-15T11:00:00Z", *resp.InstalledAt)
	})

	t.Run("installed_at nil when not installed", func(t *testing.T) {
		created := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		updated := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

		resp := NewInstallableTimestampResponse(&created, &updated, nil)

		assert.Nil(t, resp.InstalledAt)
	})
}

func TestNewPaginationMeta(t *testing.T) {
	t.Run("calculates last page correctly", func(t *testing.T) {
		meta := NewPaginationMeta(1, 10, 95)

		assert.Equal(t, 1, meta.CurrentPage)
		assert.Equal(t, 10, meta.PerPage)
		assert.Equal(t, int64(95), meta.Total)
		assert.Equal(t, 10, meta.LastPage)
	})

	t.Run("handles exact division", func(t *testing.T) {
		meta := NewPaginationMeta(1, 10, 100)

		assert.Equal(t, 10, meta.LastPage)
	})

	t.Run("handles empty results", func(t *testing.T) {
		meta := NewPaginationMeta(1, 10, 0)

		assert.Equal(t, 1, meta.LastPage)
	})

	t.Run("handles single page", func(t *testing.T) {
		meta := NewPaginationMeta(1, 10, 5)

		assert.Equal(t, 1, meta.LastPage)
	})
}

func TestNewSuccessResponse(t *testing.T) {
	t.Run("creates success response", func(t *testing.T) {
		data := map[string]string{"name": "test"}
		resp := NewSuccessResponse("Created successfully", data)

		assert.True(t, resp.Success)
		assert.Equal(t, "Created successfully", resp.Message)
		assert.Equal(t, data, resp.Data)
		assert.Nil(t, resp.Meta)
		assert.Nil(t, resp.Errors)
	})
}

func TestNewSuccessResponseWithMeta(t *testing.T) {
	t.Run("includes pagination meta", func(t *testing.T) {
		data := []string{"item1", "item2"}
		meta := NewPaginationMeta(1, 10, 50)
		resp := NewSuccessResponseWithMeta("Items retrieved", data, meta)

		assert.True(t, resp.Success)
		assert.Equal(t, "Items retrieved", resp.Message)
		assert.NotNil(t, resp.Meta)
		assert.Equal(t, 1, resp.Meta.CurrentPage)
		assert.Equal(t, int64(50), resp.Meta.Total)
	})
}

func TestNewErrorResponse(t *testing.T) {
	t.Run("creates error response", func(t *testing.T) {
		errors := map[string]string{
			"email": "Email is required",
			"name":  "Name is too short",
		}
		resp := NewErrorResponse("Validation failed", errors)

		assert.False(t, resp.Success)
		assert.Equal(t, "Validation failed", resp.Message)
		assert.Nil(t, resp.Data)
		assert.Equal(t, errors, resp.Errors)
	})
}
