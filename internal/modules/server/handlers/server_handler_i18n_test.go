package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func withProvisionLocale(t *testing.T, locale string, run func(*fiber.Ctx)) {
	t.Helper()

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		i18n.SetLocale(c, locale)
		run(c)
		return c.SendStatus(fiber.StatusNoContent)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, response.StatusCode)
	require.NoError(t, response.Body.Close())
}

func TestLocalizeProvisionStepJapaneseFixedStep(t *testing.T) {
	step := dto.ProvisionStatusStep{
		Name:        "detect_os",
		Description: "Detect the operating system, version, architecture and kernel",
		Status:      "current",
	}

	withProvisionLocale(t, i18n.LocaleJapanese, func(c *fiber.Ctx) {
		localizeProvisionStep(c, &step)
	})

	assert.Equal(t, "OS、バージョン、アーキテクチャ、カーネルを検出します", step.Description)
}

func TestLocalizeProvisionStepJapaneseDynamicService(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{name: "installed", status: "completed", want: "Redisをインストールしました"},
		{name: "installing", status: "current", want: "Redisをインストール中です"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := dto.ProvisionStatusStep{Name: "Redis", Status: tt.status}
			withProvisionLocale(t, i18n.LocaleJapanese, func(c *fiber.Ctx) {
				localizeProvisionStep(c, &step)
			})
			assert.Equal(t, tt.want, step.Description)
		})
	}
}

func TestLocalizeProvisionStatusJapaneseClassifiedError(t *testing.T) {
	status := dto.ProvisionStatusResponse{
		ErrorMessage: "Another package manager is still running on the server. This usually clears up within a few minutes — please try again shortly.",
	}

	withProvisionLocale(t, i18n.LocaleJapanese, func(c *fiber.Ctx) {
		localizeProvisionStatus(c, &status)
	})

	assert.Equal(t, "別のパッケージマネージャーがサーバーで実行中です。通常は数分で解消するため、しばらくしてからもう一度お試しください。", status.ErrorMessage)
}

func TestLocalizeProvisionStatusJapaneseUnknownStoredErrorUsesFallback(t *testing.T) {
	status := dto.ProvisionStatusResponse{
		ErrorMessage: "Hetzner returned status 503 while creating server cx23",
	}

	withProvisionLocale(t, i18n.LocaleJapanese, func(c *fiber.Ctx) {
		localizeProvisionStatus(c, &status)
	})

	assert.Equal(t, "このサーバーのプロビジョニングを完了できませんでした。もう一度試すか、問題が続く場合はサポートへお問い合わせください。", status.ErrorMessage)
}

func TestLocalizeProvisionStatusEnglishPreservesUnknownStoredError(t *testing.T) {
	const storedError = "Hetzner returned status 503 while creating server cx23"
	status := dto.ProvisionStatusResponse{ErrorMessage: storedError}

	withProvisionLocale(t, i18n.LocaleEnglish, func(c *fiber.Ctx) {
		localizeProvisionStatus(c, &status)
	})

	assert.Equal(t, storedError, status.ErrorMessage)
}
