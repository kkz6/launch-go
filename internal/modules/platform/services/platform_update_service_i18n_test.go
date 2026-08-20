package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/platform/dto"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func TestLocalizePlatformUpdateResponseUsesStableKeyAndCopiesResponse(t *testing.T) {
	source := dto.PlatformUpdateResponse{
		Key:         renameUsernameUpdateKey,
		Title:       "stale seeded title",
		Description: "stale seeded description",
	}

	localized := localizePlatformUpdateResponse(
		i18n.WithLocale(context.Background(), i18n.LocaleJapanese),
		source,
	)

	assert.Equal(t, "デフォルトユーザー名をcaptainへ変更", localized.Title)
	assert.Equal(t, "プロビジョニング済みサーバーのデフォルトUNIXユーザー名を`launcher`から`captain`へ変更します。この更新では、システムユーザーとホームディレクトリを移行し、サーバー上の関連設定ファイル（Caddyfile、systemdサービス、crontab）を更新します。\n\nこの処理は安全で、繰り返し実行しても同じ結果になります。`captain`ユーザーがすでに存在するサーバーは自動的にスキップされます。", localized.Description)
	assert.Equal(t, "stale seeded title", source.Title)
	assert.Equal(t, "stale seeded description", source.Description)
}

func TestLocalizePlatformUpdateResponsePreservesUnknownContent(t *testing.T) {
	source := dto.PlatformUpdateResponse{
		Key:         "customer_authored_update",
		Title:       "Customer-authored title",
		Description: "Customer-authored description",
	}

	localized := localizePlatformUpdateResponse(
		i18n.WithLocale(context.Background(), i18n.LocaleJapanese),
		source,
	)

	assert.Equal(t, source, localized)
}
