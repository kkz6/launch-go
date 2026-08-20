package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func TestLocalizeCreateServerOptionsClonesStableLabels(t *testing.T) {
	source := dto.CreateServerOptionsResponse{ServerTypes: map[string]string{
		"php":          "PHP Application Server",
		"database":     "Database Server",
		"loadbalancer": "Load Balancer",
		"docker":       "Docker Application Server",
		"custom":       "Customer-defined server",
	}}

	localized := localizeCreateServerOptions(
		i18n.WithLocale(context.Background(), i18n.LocaleJapanese),
		source,
	)

	assert.Equal(t, "PHPアプリケーションサーバー", localized.ServerTypes["php"])
	assert.Equal(t, "データベースサーバー", localized.ServerTypes["database"])
	assert.Equal(t, "ロードバランサー", localized.ServerTypes["loadbalancer"])
	assert.Equal(t, "Dockerアプリケーションサーバー", localized.ServerTypes["docker"])
	assert.Equal(t, "Customer-defined server", localized.ServerTypes["custom"])
	assert.Equal(t, "PHP Application Server", source.ServerTypes["php"])
}

func TestLocalizedSoftwareLogNameUsesStableSoftware(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)
	tests := map[string]string{
		"mysql80":      "MySQL 8.0ログ",
		"postgresql16": "PostgreSQL 16ログ",
		"redis":        "Redisログ",
		"php56":        "PHP 5.6ログ",
		"php70":        "PHP 7.0ログ",
		"php71":        "PHP 7.1ログ",
		"php72":        "PHP 7.2ログ",
		"php73":        "PHP 7.3ログ",
		"php74":        "PHP 7.4ログ",
		"php80":        "PHP 8.0ログ",
		"php81":        "PHP 8.1ログ",
		"php82":        "PHP 8.2ログ",
		"php83":        "PHP 8.3ログ",
		"php84":        "PHP 8.4ログ",
		"caddy2":       "Caddy 2ログ",
		"caddy2_lb":    "Caddy 2（ロードバランサー）ログ",
		"supervisor":   "Supervisorログ",
	}

	for software, expected := range tests {
		t.Run(software, func(t *testing.T) {
			assert.Equal(t, expected, localizedSoftwareLogName(ctx, servertypes.Software(software)))
		})
	}
}
