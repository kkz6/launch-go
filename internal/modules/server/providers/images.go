package providers

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

type ImageFinding struct {
	OS     string
	Image  string
	Reason string
}

type imageLookup interface {
	LookupImage(ctx context.Context, credentials map[string]any, image string) error
}

// ValidateImages checks configured images against the provider. Missing
// configuration or an unavailable lookup is reported as an unverified image.
func ValidateImages(ctx context.Context, provider Provider, cfg config.ProviderConfig, creds map[string]any) []ImageFinding {
	lookup, ok := provider.(imageLookup)
	if !ok {
		return []ImageFinding{{Reason: "provider has no live image lookup"}}
	}
	var findings []ImageFinding
	check := func(os, image string) {
		if image == "" {
			findings = append(findings, ImageFinding{OS: os, Reason: "missing configured image identifier"})
		} else if err := lookup.LookupImage(ctx, creds, image); err != nil {
			findings = append(findings, ImageFinding{OS: os, Image: image, Reason: err.Error()})
		}
	}
	if provider.Type() == types.ProviderAWS {
		region := GetStringField(creds, "region", defaultAWSValidationRegion)
		for _, os := range types.AllOperatingSystems() {
			check(fmt.Sprintf("%s (%s)", os, region), AWSSSMParameterByOS[os.String()])
		}
		return findings
	}
	if len(cfg.Images) == 0 {
		return []ImageFinding{{Reason: "no configured images"}}
	}
	for _, os := range slices.Sorted(maps.Keys(cfg.Images)) {
		image, _ := cfg.Images[os].(string)
		check(os, image)
	}
	for _, os := range types.AllOperatingSystems() {
		if _, exists := cfg.Images[os.String()]; !exists {
			check(os.String(), "")
		}
	}
	return findings
}
