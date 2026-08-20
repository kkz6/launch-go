package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func TestLocalizedFileMetadataUsesStableFileType(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)
	tests := []struct {
		fileType        sitetypes.SiteFileType
		wantName        string
		wantDescription string
	}{
		{
			fileType:        sitetypes.SiteFileCaddyfile,
			wantName:        "Caddyfile",
			wantDescription: "Caddyの設定ファイルです。リクエストの処理やTLS証明書など、サイトの動作を設定します。",
		},
		{
			fileType:        sitetypes.SiteFileEnvironment,
			wantName:        "環境ファイル",
			wantDescription: "サイトの環境ファイルです。サイトで利用できる環境変数が含まれています。",
		},
		{
			fileType:        sitetypes.SiteFileComposerAuth,
			wantName:        "Composer auth.json",
			wantDescription: "Composerのauth.jsonファイルです。GitHubやBitbucketなどからプライベートパッケージをインストールするための認証情報が含まれています。",
		},
		{
			fileType:        sitetypes.SiteFileWordpressConfig,
			wantName:        "WordPress設定ファイル",
			wantDescription: "WordPressの設定ファイルです。データベースの認証情報やその他の設定が含まれています。",
		},
		{
			fileType:        sitetypes.SiteFileLaravelLog,
			wantName:        "Laravelログ",
			wantDescription: "Laravelフレームワークが作成する既定のログファイルです。",
		},
		{
			fileType:        sitetypes.SiteFileCaddyLog,
			wantName:        "Caddyログ",
			wantDescription: "サイトへのすべてのリクエストが記録されるCaddyログです。",
		},
	}

	for _, test := range tests {
		t.Run(test.fileType.String(), func(t *testing.T) {
			name, description := localizedFileMetadata(ctx, test.fileType)
			assert.Equal(t, test.wantName, name)
			assert.Equal(t, test.wantDescription, description)
		})
	}
}

func TestLocalizedFileMetadataPreservesUnknownValues(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)
	name, description := localizedFileMetadata(ctx, sitetypes.SiteFileType("customer_file"))

	assert.Equal(t, "customer_file", name)
	assert.Empty(t, description)
}
