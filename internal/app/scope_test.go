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
