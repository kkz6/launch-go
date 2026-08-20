package i18n

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectAcceptLanguage(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		fallback string
		want     string
	}{
		{name: "empty", fallback: LocaleEnglish, want: LocaleEnglish},
		{name: "Japanese", header: "ja", fallback: LocaleEnglish, want: LocaleJapanese},
		{name: "Japanese region", header: "ja-JP", fallback: LocaleEnglish, want: LocaleJapanese},
		{name: "quality weights", header: "en;q=0.4, ja-JP;q=0.9", fallback: LocaleEnglish, want: LocaleJapanese},
		{name: "English preferred", header: "ja;q=0.2, en-US;q=0.8", fallback: LocaleJapanese, want: LocaleEnglish},
		{name: "skip unsupported candidate", header: "fr-FR, ja;q=0.8", fallback: LocaleEnglish, want: LocaleJapanese},
		{name: "unsupported", header: "fr-FR", fallback: LocaleEnglish, want: LocaleEnglish},
		{name: "wildcard uses fallback", header: "*", fallback: LocaleEnglish, want: LocaleEnglish},
		{name: "zero quality is excluded", header: "ja;q=0, en;q=0.5", fallback: LocaleJapanese, want: LocaleEnglish},
		{name: "only zero quality uses fallback", header: "ja;q=0", fallback: LocaleEnglish, want: LocaleEnglish},
		{name: "malformed", header: "ja;q=not-a-number", fallback: LocaleEnglish, want: LocaleEnglish},
		{name: "invalid fallback", fallback: "jp", want: LocaleEnglish},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectAcceptLanguage(tt.header, tt.fallback))
		})
	}
}

func TestNormalizeAllowListsLocales(t *testing.T) {
	locale, ok := Normalize("JA_jp")
	assert.True(t, ok)
	assert.Equal(t, LocaleJapanese, locale)

	_, ok = Normalize("jp")
	assert.False(t, ok)
	_, ok = Normalize("fr")
	assert.False(t, ok)
}

func TestTranslateFallsBackSafely(t *testing.T) {
	assert.Equal(t, "unowned message", Translate(LocaleJapanese, "unowned message"))
	assert.Equal(t, "Missing required parameter: id", Translate(LocaleEnglish, "Missing required parameter: %s", "id"))
	assert.NotEqual(t, "Validation failed", Translate(LocaleJapanese, "Validation failed"))
}

func TestLocalePropagatesThroughStandardContext(t *testing.T) {
	ctx := WithLocale(context.Background(), "ja-JP")

	assert.Equal(t, LocaleJapanese, LocaleFromContext(ctx))
	assert.Equal(t, "DNSの検索に失敗しました。", TContext(ctx, "DNS lookup failed."))
	assert.Equal(t, LocaleEnglish, LocaleFromContext(context.Background()))
}

func TestCatalogsHaveKeyAndPlaceholderParity(t *testing.T) {
	placeholder := regexp.MustCompile(`%[sdq]`)
	en := catalogs[LocaleEnglish]
	ja := catalogs[LocaleJapanese]

	for key, english := range en {
		japanese, ok := ja[key]
		assert.Truef(t, ok, "Japanese catalog missing %q", key)
		assert.Equalf(t, placeholder.FindAllString(english, -1), placeholder.FindAllString(japanese, -1), "placeholder mismatch for %q", key)
	}
	for key := range ja {
		_, ok := en[key]
		assert.Truef(t, ok, "English catalog missing %q", key)
	}
}
