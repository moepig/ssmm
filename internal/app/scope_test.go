package app

import (
	"context"
	"testing"

	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type scopeRuntime struct{}

func (scopeRuntime) EC2(context.Context, string, target.TargetQuery, func([]target.Instance) error) error {
	return nil
}
func (scopeRuntime) SSM(context.Context, string, func([]inventory.SSMRecord) error) error { return nil }
func (scopeRuntime) EnabledRegions(context.Context, string) ([]string, error) {
	return []string{"us-east-1"}, nil
}

type allRegionsRuntime struct {
	discovery string
}

func (r *allRegionsRuntime) EC2(context.Context, string, target.TargetQuery, func([]target.Instance) error) error {
	return nil
}
func (r *allRegionsRuntime) SSM(context.Context, string, func([]inventory.SSMRecord) error) error {
	return nil
}
func (r *allRegionsRuntime) EnabledRegions(_ context.Context, discovery string) ([]string, error) {
	r.discovery = discovery
	return []string{"us-west-2", "us-east-1"}, nil
}

func TestRegionOverridePrecedesInvalidSavedScope(t *testing.T) {
	bad := []string{"not-a-region"}
	scope, err := ResolveScope(context.Background(), scopeRuntime{}, target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}, SettingsSnapshot{Profiles: map[string]ProfileSettings{"prod": {Regions: &bad}}}, "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.Regions) != 1 || scope.Regions[0] != "us-east-1" {
		t.Fatalf("unexpected scope: %#v", scope)
	}
	if _, err := ResolveScope(context.Background(), scopeRuntime{}, target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}, SettingsSnapshot{Profiles: map[string]ProfileSettings{"prod": {Regions: &bad}}}, ""); err == nil {
		t.Fatal("invalid saved scope was accepted without override")
	}
}

func TestResolveScopeUsesSortedAllRegionsWithoutDiscoveryInScope(t *testing.T) {
	runtime := &allRegionsRuntime{}
	scope, err := ResolveScope(context.Background(), runtime, target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}, SettingsSnapshot{Profiles: map[string]ProfileSettings{"prod": {}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.discovery != "" {
		t.Fatalf("discovery region = %q, want empty", runtime.discovery)
	}
	if got, want := scope.Regions, []string{"us-east-1", "us-west-2"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("regions = %#v, want %#v", got, want)
	}
	if err := scope.Validate(); err != nil {
		t.Fatal(err)
	}
}
