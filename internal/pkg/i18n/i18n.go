// Package i18n resolves request locales and translates API-facing messages.
//
// Locale values are deliberately allow-listed. Accept-Language input is never
// used as a file name or returned verbatim in a response header.
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/text/language"
)

const (
	LocaleAuto     = "auto"
	LocaleEnglish  = "en"
	LocaleJapanese = "ja"
	DefaultLocale  = LocaleEnglish

	localeContextKey         = "launch.locale"
	detectedLocaleContextKey = "launch.detected_locale"
)

type standardContextKey struct{}

var localeStandardContextKey standardContextKey

var (
	supportedLocales = []string{LocaleEnglish, LocaleJapanese}

	//go:embed locales/en.json locales/ja.json
	localeFiles embed.FS

	catalogs = mustLoadCatalogs()
)

// mustLoadCatalogs loads the embedded translation catalogs. Invalid catalogs
// are a build/deploy error, so startup fails rather than serving partial copy.
func mustLoadCatalogs() map[string]map[string]string {
	loaded := make(map[string]map[string]string, len(supportedLocales))
	for _, locale := range supportedLocales {
		path := "locales/" + locale + ".json"
		contents, err := localeFiles.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("load locale catalog %s: %v", path, err))
		}

		catalog := make(map[string]string)
		if err := json.Unmarshal(contents, &catalog); err != nil {
			panic(fmt.Sprintf("decode locale catalog %s: %v", path, err))
		}
		loaded[locale] = catalog
	}
	return loaded
}

// Normalize returns the canonical supported locale for a language tag.
// Region variants such as ja-JP are canonicalized to ja.
func Normalize(raw string) (string, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "_", "-"))
	if raw == "" {
		return "", false
	}

	tag, err := language.Parse(raw)
	if err != nil {
		return "", false
	}
	return normalizeTag(tag)
}

func normalizeTag(tag language.Tag) (string, bool) {
	base, _ := tag.Base()
	switch base.String() {
	case LocaleEnglish:
		return LocaleEnglish, true
	case LocaleJapanese:
		return LocaleJapanese, true
	default:
		return "", false
	}
}

// DetectAcceptLanguage selects a supported locale from an Accept-Language
// header, respecting quality weights. Empty, malformed, and unsupported
// headers use fallback (or English when fallback is invalid).
func DetectAcceptLanguage(header, fallback string) string {
	canonicalFallback, ok := Normalize(fallback)
	if !ok {
		canonicalFallback = DefaultLocale
	}
	if strings.TrimSpace(header) == "" {
		return canonicalFallback
	}

	tags, _, err := language.ParseAcceptLanguage(header)
	if err != nil || len(tags) == 0 {
		return canonicalFallback
	}
	// ParseAcceptLanguage returns candidates in descending quality order. Skip
	// unsupported candidates instead of letting a matcher collapse (for
	// example) high-quality French to English ahead of lower-quality Japanese.
	for _, tag := range tags {
		if canonical, supported := normalizeTag(tag); supported {
			return canonical
		}
	}
	return canonicalFallback
}

// SetDetectedLocale records the header-derived locale and makes it the active
// locale until an authenticated user preference overrides it.
func SetDetectedLocale(c *fiber.Ctx, locale string) {
	canonical, ok := Normalize(locale)
	if !ok {
		canonical = DefaultLocale
	}
	c.Locals(detectedLocaleContextKey, canonical)
	SetLocale(c, canonical)
}

// DetectedLocale returns the header-derived locale for the request.
func DetectedLocale(c *fiber.Ctx) string {
	if c != nil {
		if locale, ok := c.Locals(detectedLocaleContextKey).(string); ok {
			if canonical, supported := Normalize(locale); supported {
				return canonical
			}
		}
	}
	return DefaultLocale
}

// SetLocale makes a supported locale active for the remainder of the request.
func SetLocale(c *fiber.Ctx, locale string) {
	if c == nil {
		return
	}
	canonical, ok := Normalize(locale)
	if !ok {
		canonical = DefaultLocale
	}
	c.Locals(localeContextKey, canonical)
	c.Set("Content-Language", canonical)
}

// ApplyPreference applies a persisted locale when it is supported. A nil or
// invalid preference means automatic mode and restores the detected locale.
func ApplyPreference(c *fiber.Ctx, preferred *string) string {
	resolved := DetectedLocale(c)
	if preferred != nil {
		if canonical, ok := Normalize(*preferred); ok {
			resolved = canonical
		}
	}
	SetLocale(c, resolved)
	return resolved
}

// Locale returns the active request locale. Callers without locale middleware
// (notably existing tests) retain the historical English behavior.
func Locale(c *fiber.Ctx) string {
	if c != nil {
		if locale, ok := c.Locals(localeContextKey).(string); ok {
			if canonical, supported := Normalize(locale); supported {
				return canonical
			}
		}
	}
	return DefaultLocale
}

// WithLocale returns a standard context carrying a canonical locale. Fiber's
// RequestCtx exposes Locals through context.Context.Value, so service methods
// normally receive the request locale without an adapter; this helper is also
// useful for background callers and focused service tests.
func WithLocale(ctx context.Context, locale string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	canonical, ok := Normalize(locale)
	if !ok {
		canonical = DefaultLocale
	}
	return context.WithValue(ctx, localeStandardContextKey, canonical)
}

// LocaleFromContext returns the request locale from a standard context.
// Missing values preserve the historical English behavior.
func LocaleFromContext(ctx context.Context) string {
	if ctx != nil {
		if locale, ok := ctx.Value(localeStandardContextKey).(string); ok {
			if canonical, supported := Normalize(locale); supported {
				return canonical
			}
		}
		// Fiber's RequestCtx exposes request Locals through Value using the
		// original string key, so retain that lookup for handler/service calls.
		if locale, ok := ctx.Value(localeContextKey).(string); ok {
			if canonical, supported := Normalize(locale); supported {
				return canonical
			}
		}
	}
	return DefaultLocale
}

// Translate looks up an exact English source message and optionally applies
// fmt-style arguments. Missing Japanese entries fall back through the English
// catalog and finally to the source message itself.
func Translate(locale, message string, args ...any) string {
	canonical, ok := Normalize(locale)
	if !ok {
		canonical = DefaultLocale
	}

	translated := message
	if catalog := catalogs[canonical]; catalog != nil {
		if value, found := catalog[message]; found && value != "" {
			translated = value
		} else if value, found := catalogs[DefaultLocale][message]; found && value != "" {
			translated = value
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(translated, args...)
	}
	return translated
}

// T translates a message using the active request locale.
func T(c *fiber.Ctx, message string, args ...any) string {
	return Translate(Locale(c), message, args...)
}

// TContext translates a message using a locale propagated to a service's
// context.Context. This keeps lower-level response DTOs independent of Fiber.
func TContext(ctx context.Context, message string, args ...any) string {
	return Translate(LocaleFromContext(ctx), message, args...)
}

// TranslateErrors returns a translated copy of a field-validation map.
func TranslateErrors(c *fiber.Ctx, errors map[string][]string) map[string][]string {
	if errors == nil {
		return nil
	}
	translated := make(map[string][]string, len(errors))
	for field, messages := range errors {
		translated[field] = make([]string, 0, len(messages))
		for _, message := range messages {
			translated[field] = append(translated[field], T(c, message))
		}
	}
	return translated
}

// AddVary merges a response Vary token without clobbering values set by CORS
// or another middleware.
func AddVary(c *fiber.Ctx, value string) {
	if c == nil || strings.TrimSpace(value) == "" {
		return
	}
	existing := c.GetRespHeader("Vary")
	for _, token := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(token), value) {
			return
		}
	}
	if existing == "" {
		c.Set("Vary", value)
		return
	}
	c.Set("Vary", existing+", "+value)
}
