package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

// GetSiteHookDefaults exists so the create dialog can show the same script
// SiteService.Create would apply if the request left hooks unset — for
// Laravel, that script is composer install and the artisan cache commands,
// not an empty placeholder. If this drifts from what Create actually reads,
// the dialog shows one script and the site gets another.
func TestGetSiteHookDefaultsMatchesCreateDefaults(t *testing.T) {
	tests := []struct {
		name         string
		siteType     sitetypes.SiteType
		zeroDowntime bool
	}{
		{name: "laravel non-zero-downtime", siteType: sitetypes.SiteTypeLaravel, zeroDowntime: false},
		{name: "laravel zero-downtime", siteType: sitetypes.SiteTypeLaravel, zeroDowntime: true},
		{name: "static non-zero-downtime", siteType: sitetypes.SiteTypeStatic, zeroDowntime: false},
		{name: "static zero-downtime", siteType: sitetypes.SiteTypeStatic, zeroDowntime: true},
		{name: "wordpress", siteType: sitetypes.SiteTypeWordpress, zeroDowntime: false},
		{name: "phpmyadmin", siteType: sitetypes.SiteTypePhpMyAdmin, zeroDowntime: false},
		{name: "generic", siteType: sitetypes.SiteTypeGeneric, zeroDowntime: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.siteType.GetDefaultAttributes(tc.zeroDowntime)
			got := GetSiteHookDefaults(tc.siteType, tc.zeroDowntime)

			asString := func(v any) string {
				s, _ := v.(string)
				return s
			}

			assert.Equal(t, asString(want["hook_before_updating_repository"]), got.HookBeforeUpdatingRepository)
			assert.Equal(t, asString(want["hook_after_updating_repository"]), got.HookAfterUpdatingRepository)
			assert.Equal(t, asString(want["hook_before_making_current"]), got.HookBeforeMakingCurrent)
			assert.Equal(t, asString(want["hook_after_making_current"]), got.HookAfterMakingCurrent)
		})
	}
}

// Laravel's non-zero-downtime default is not a placeholder — it's the
// composer install + artisan cache script that actually builds the site.
// This pins its presence so a refactor can't quietly turn it back into "".
func TestGetSiteHookDefaultsLaravelCarriesTheBuildScript(t *testing.T) {
	defaults := GetSiteHookDefaults(sitetypes.SiteTypeLaravel, false)

	assert.Contains(t, defaults.HookAfterUpdatingRepository, "composer install")
	assert.Contains(t, defaults.HookAfterUpdatingRepository, "artisan config:cache")
	assert.Contains(t, defaults.HookBeforeUpdatingRepository, "artisan down")
}

// For zero-downtime, the release is built once before it goes live, so the
// script moves to "before making current" and "after updating repository"
// is empty — updating the mirror on every deploy must not rebuild it.
func TestGetSiteHookDefaultsLaravelZeroDowntimeMovesTheScript(t *testing.T) {
	defaults := GetSiteHookDefaults(sitetypes.SiteTypeLaravel, true)

	assert.Empty(t, defaults.HookAfterUpdatingRepository)
	assert.Contains(t, defaults.HookBeforeMakingCurrent, "composer install")
}

// Types with no build step get exactly empty hooks — the create dialog's
// "leave blank to use the defaults" note has to actually be true for these.
func TestGetSiteHookDefaultsEmptyForTypesWithNoBuildStep(t *testing.T) {
	for _, siteType := range []sitetypes.SiteType{
		sitetypes.SiteTypeWordpress,
		sitetypes.SiteTypePhpMyAdmin,
		sitetypes.SiteTypeGeneric,
	} {
		defaults := GetSiteHookDefaults(siteType, false)
		assert.Empty(t, defaults.HookBeforeUpdatingRepository, "%s", siteType)
		assert.Empty(t, defaults.HookAfterUpdatingRepository, "%s", siteType)
		assert.Empty(t, defaults.HookBeforeMakingCurrent, "%s", siteType)
		assert.Empty(t, defaults.HookAfterMakingCurrent, "%s", siteType)
	}
}

func TestGetSiteHookDefaultsUnknownTypeIsEmpty(t *testing.T) {
	defaults := GetSiteHookDefaults(sitetypes.SiteType("not-a-real-type"), false)
	assert.Equal(t, SiteHookDefaultsResponse{}, defaults)
}
