package app

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

func ResolveScope(ctx context.Context, runtime AWSRuntime, selection target.ProfileSelection, settings SettingsSnapshot, regionOverride string) (inventory.SearchScope, error) {
	if runtime == nil {
		return inventory.SearchScope{}, fmt.Errorf("AWS runtime is nil")
	}
	profile := settings.Profile(selection.Name)
	if regionOverride == "" {
		if err := validateProfileScope(profile); err != nil {
			return inventory.SearchScope{}, err
		}
	} else if err := validateSSHSettings(profile); err != nil {
		return inventory.SearchScope{}, err
	}
	var configured []string
	source := "all"
	if regionOverride != "" {
		if !isCommercialRegion(regionOverride) {
			return inventory.SearchScope{}, fmt.Errorf("invalid region %q", regionOverride)
		}
		configured = []string{regionOverride}
		source = "--region"
	} else if profile.Regions != nil {
		configured = append([]string(nil), (*profile.Regions)...)
		source = "profiles." + selection.Name + ".regions"
		if len(configured) == 0 {
			return inventory.SearchScope{}, fmt.Errorf("configured region list is empty")
		}
	}
	discovery := ""
	if len(configured) > 0 {
		discovery = configured[0]
	}
	enabled, err := runtime.EnabledRegions(ctx, discovery)
	if err != nil {
		return inventory.SearchScope{}, fmt.Errorf("resolve enabled regions: %w", err)
	}
	if len(configured) == 0 {
		configured = append([]string(nil), enabled...)
		sort.Strings(configured)
	} else {
		allowed := make(map[string]bool, len(enabled))
		for _, r := range enabled {
			allowed[r] = true
		}
		for _, r := range configured {
			if !allowed[r] {
				return inventory.SearchScope{}, fmt.Errorf("region %q is not enabled for this account", r)
			}
		}
	}
	if len(configured) == 0 {
		return inventory.SearchScope{}, fmt.Errorf("account has no enabled regions")
	}
	return inventory.SearchScope{Regions: configured, Source: source}, nil
}

var regionRE = regexp.MustCompile(`^[a-z]{2}(?:-gov)?-[a-z]+-\d+$`)

func isCommercialRegion(region string) bool {
	return regionRE.MatchString(region) && !strings.HasPrefix(region, "cn-") && !strings.Contains(region, "-gov-")
}

func validateProfileScope(profile ProfileSettings) error {
	if profile.Regions != nil {
		if len(*profile.Regions) == 0 {
			return fmt.Errorf("configured region list is empty")
		}
		for _, region := range *profile.Regions {
			if !isCommercialRegion(region) {
				return fmt.Errorf("invalid configured region %q", region)
			}
		}
	}
	return profile.SSH.Validate()
}

func validateSSHSettings(profile ProfileSettings) error {
	return profile.SSH.Validate()
}
