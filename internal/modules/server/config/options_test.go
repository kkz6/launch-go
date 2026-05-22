package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
)

// allActiveOSes mirrors the values the UI lets users pick. Provider configs
// MUST cover every one of these; missing entries silently fall back to the
// provider's default in code, which is how we ended up shipping retired
// numeric snapshot IDs on DigitalOcean. This test fails loudly so the next
// time we add an OS (or drop one) every provider stays in sync.
func allActiveOSes() []string {
	out := make([]string, 0, len(servertypes.AllOperatingSystems()))
	for _, os := range servertypes.AllOperatingSystems() {
		out = append(out, os.String())
	}
	return out
}

// providerSupportedOSes captures, per provider, which OSes we still have
// working upstream images for. When an upstream retires an image (DO killed
// `ubuntu-20-04-x64`) it drops out of the provider's entry here AND out of
// the provider's Images map in options.go — and the test asserts the two
// stay in sync, so a future config edit can't silently leave a half-removed
// OS visible to users.
var providerSupportedOSes = map[string][]string{
	"digitalocean": {"ubuntu_22", "ubuntu_24"}, // ubuntu_20 retired by DO
	"hetzner":      {"ubuntu_20", "ubuntu_22", "ubuntu_24"},
	"linode":       {"ubuntu_20", "ubuntu_22", "ubuntu_24"},
	"vultr":        {"ubuntu_20", "ubuntu_22", "ubuntu_24"},
}

func TestProviderConfigs_MatchSupportedOSes(t *testing.T) {
	configs := GetProviderConfigs()

	// AWS is region-sharded — its Images map is keyed by region, not by OS.
	// We assert AWS coverage separately in TestAWSConfig_ImagesCoverAllRegions.
	for provider, cfg := range configs {
		if provider == "aws" {
			continue
		}
		t.Run(provider, func(t *testing.T) {
			supported, ok := providerSupportedOSes[provider]
			require.True(t, ok, "providerSupportedOSes is missing an entry for %q — add one or remove the provider", provider)

			// Every supported OS must have a non-empty image string.
			for _, os := range supported {
				img, present := cfg.Images[os]
				require.True(t, present, "%s missing image entry for supported OS %s", provider, os)
				s, isStr := img.(string)
				require.True(t, isStr, "%s image entry for %s must be a string", provider, os)
				assert.NotEmpty(t, s, "%s image entry for %s must not be empty", provider, os)
			}

			// And no extras — if a provider has an image entry for an OS
			// not in the supported list, it's leftover config from a
			// retired image.
			supportedSet := make(map[string]struct{}, len(supported))
			for _, os := range supported {
				supportedSet[os] = struct{}{}
			}
			for os := range cfg.Images {
				_, ok := supportedSet[os]
				assert.True(t, ok,
					"%s has image entry for %s but it's not in providerSupportedOSes — either it's retired upstream (remove it) or add it to the supported list",
					provider, os)
			}
		})
	}
}

// TestDigitalOceanImages_AreSlugs guards against the regression that took
// down provisioning: configuring numeric snapshot IDs ("168977420") on DO
// instead of slugs ("ubuntu-24-04-x64"). DO retires the snapshot IDs over
// time but keeps the slugs as stable references — so we should never
// configure a pure-numeric value here.
func TestDigitalOceanImages_AreSlugs(t *testing.T) {
	configs := GetProviderConfigs()
	do, ok := configs["digitalocean"]
	require.True(t, ok)

	for osKey, img := range do.Images {
		t.Run(osKey, func(t *testing.T) {
			s, _ := img.(string)
			assert.False(t, isAllDigits(s),
				"DO image %q for %q is a numeric snapshot ID — use a slug like 'ubuntu-24-04-x64' instead. Snapshot IDs are retired by DO and break provisioning.", s, osKey)
			assert.True(t, strings.Contains(s, "-"),
				"DO image %q for %q does not look like a slug; expected a hyphenated identifier", s, osKey)
		})
	}
}

// TestAWSConfig_ImagesCoverAllRegions asserts every region in the AWS region
// list has an AMI entry for every active OS. Without this, picking certain
// regions silently returns "" from GetAWSImageForRegion which the upstream
// rejects with a confusing error.
func TestAWSConfig_ImagesCoverAllRegions(t *testing.T) {
	aws := getAWSConfig()

	regions := make([]string, 0, len(aws.Regions))
	for _, r := range aws.Regions {
		regions = append(regions, r.Value)
	}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			entry, ok := aws.Images[region]
			require.True(t, ok, "AWS region %s has no AMI map", region)

			osMap, ok := entry.(map[string]string)
			require.True(t, ok, "AWS images[%s] must be map[string]string", region)

			for _, os := range allActiveOSes() {
				ami, ok := osMap[os]
				require.True(t, ok, "AWS region %s missing AMI for %s", region, os)
				assert.True(t, strings.HasPrefix(ami, "ami-"),
					"AWS images[%s][%s] = %q does not look like an AMI ID", region, os, ami)
			}
		})
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
