package app

import (
	"context"
	"fmt"

	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type Service struct {
	Settings SettingsStore
	Profiles ProfileChecker
	AWS      AWSFactory
	Session  SessionPlanner
	SSH      SSHPlanner
	Runner   Runner
}

type SearchRequest struct {
	// CLI フラグまたは環境変数から選択した AWS プロファイル。明示フラグは保存設定より優先する。
	AWSProfile target.ProfileSelection
	// 検索リージョンと SSH の既定値を選択する ssmm プロファイル。
	Profile target.ProfileSelection
	Target  string
	Tags    []target.TagFilter
	Filter  string
	Region  string
}

type SearchResult struct {
	AWSProfile target.ProfileSelection
	Profile    target.ProfileSelection
	Settings   SettingsSnapshot
	Scope      inventory.SearchScope
	Snapshot   inventory.InventorySnapshot
	Target     target.ResolvedTarget
	Query      target.TargetQuery
	Filter     string
}

// SearchRun contains the immutable search inputs and the live inventory
// generation used by interactive selection.
type SearchRun struct {
	AWSProfile target.ProfileSelection
	Profile    target.ProfileSelection
	Settings   SettingsSnapshot
	Scope      inventory.SearchScope
	Query      target.TargetQuery
	Filter     string
	Run        *inventory.Run
	Generation uint64
}

// StartInventory prepares one search and returns its snapshots as they arrive.
// The caller must cancel the run when it no longer needs the stream and wait
// for Run.Done before starting another generation.
func (s Service) StartInventory(ctx context.Context, req SearchRequest, generation uint64) (SearchRun, error) {
	if s.AWS == nil || s.Profiles == nil {
		return SearchRun{}, fmt.Errorf("service dependencies are incomplete")
	}
	settings, err := s.ReadSettings(ctx, req.Profile)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrInvalid, Operation: "read settings", Err: err}
	}
	awsProfile := settings.AWSSelection(req.Profile.Name, req.AWSProfile)
	if err := s.Profiles.Validate(ctx, awsProfile); err != nil {
		return SearchRun{}, &AppError{Kind: ErrAuth, Operation: "validate AWS profile", Profile: awsProfile.Name, Err: err}
	}
	q, err := target.ParseQuery(req.Target, req.Tags)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrInvalid, Operation: "parse target", Err: err}
	}
	runtime, err := s.AWS.Open(ctx, awsProfile)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrAuth, Operation: "open AWS", Profile: awsProfile.Name, Err: err}
	}
	scope, err := ResolveScope(ctx, runtime, req.Profile, settings, req.Region)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrFetch, Operation: "resolve search scope", Profile: req.Profile.Name, Err: err}
	}
	run, err := inventory.Start(ctx, generation, scope, q, runtime)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrFetch, Operation: "start search", Profile: req.Profile.Name, Err: err}
	}
	return SearchRun{Profile: req.Profile, AWSProfile: awsProfile, Settings: settings, Scope: scope, Query: q, Filter: req.Filter, Run: run, Generation: generation}, nil
}

func (s Service) Inventory(ctx context.Context, req SearchRequest) (SettingsSnapshot, inventory.SearchScope, inventory.InventorySnapshot, target.TargetQuery, error) {
	search, err := s.StartInventory(ctx, req, 1)
	if err != nil {
		return SettingsSnapshot{}, inventory.SearchScope{}, inventory.InventorySnapshot{}, target.TargetQuery{}, &AppError{Kind: ErrFetch, Operation: "search instances", Profile: req.Profile.Name, Err: err}
	}
	var snapshot inventory.InventorySnapshot
	for current := range search.Run.Snapshots {
		snapshot = current
	}
	<-search.Run.Done
	if snapshot.Revision == 0 {
		return SettingsSnapshot{}, inventory.SearchScope{}, inventory.InventorySnapshot{}, target.TargetQuery{}, &AppError{Kind: ErrFetch, Operation: "search instances", Profile: req.Profile.Name, Err: fmt.Errorf("inventory ended without a snapshot")}
	}
	return search.Settings, search.Scope, snapshot, search.Query, nil
}

// ssmm プロファイルが選択されていない場合は設定を読み込まず、空の設定を返す。
func (s Service) ReadSettings(ctx context.Context, profile target.ProfileSelection) (SettingsSnapshot, error) {
	if profile == (target.ProfileSelection{}) {
		return SettingsSnapshot{}, nil
	}
	if err := target.ValidateSSMMProfileName(profile.Name); err != nil {
		return SettingsSnapshot{}, err
	}
	switch profile.Source {
	case target.ProfileConfig, target.ProfileFlag, target.ProfileHost, target.ProfileDefault:
	default:
		return SettingsSnapshot{}, fmt.Errorf("invalid ssmm profile source %q", profile.Source)
	}
	if profile.Source == target.ProfileDefault && profile.Name != "default" {
		return SettingsSnapshot{}, fmt.Errorf("default ssmm profile source requires profile name default")
	}
	if s.Settings == nil {
		return SettingsSnapshot{}, fmt.Errorf("settings store is unavailable")
	}
	settings, err := s.Settings.Read(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if _, ok := settings.Profiles[profile.Name]; !ok {
		return SettingsSnapshot{}, fmt.Errorf("ssmm profile %q is not configured; run ssmm init --ssmm-profile %s", profile.Name, profile.Name)
	}
	return settings, nil
}
