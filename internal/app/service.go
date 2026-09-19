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
	Profile        target.ProfileSelection
	Target         string
	Tags           []target.TagFilter
	Filter         string
	Region         string
	NonInteractive bool
}

type SearchResult struct {
	Profile  target.ProfileSelection
	Settings SettingsSnapshot
	Scope    inventory.SearchScope
	Snapshot inventory.InventorySnapshot
	Target   target.ResolvedTarget
	Query    target.TargetQuery
	Filter   string
}

// SearchRun contains the immutable search inputs and the live inventory
// generation used by interactive selection.
type SearchRun struct {
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
	if s.Settings == nil || s.AWS == nil || s.Profiles == nil {
		return SearchRun{}, fmt.Errorf("service dependencies are incomplete")
	}
	if err := s.Profiles.Validate(ctx, req.Profile); err != nil {
		return SearchRun{}, &AppError{Kind: ErrAuth, Operation: "validate profile", Profile: req.Profile.Name, Err: err}
	}
	settings, err := s.Settings.Read(ctx)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrInvalid, Operation: "read settings", Err: err}
	}
	q, err := target.ParseQuery(req.Target, req.Tags)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrInvalid, Operation: "parse target", Err: err}
	}
	runtime, err := s.AWS.Open(ctx, req.Profile)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrAuth, Operation: "open AWS", Profile: req.Profile.Name, Err: err}
	}
	scope, err := ResolveScope(ctx, runtime, req.Profile, settings, req.Region)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrFetch, Operation: "resolve search scope", Profile: req.Profile.Name, Err: err}
	}
	run, err := inventory.Start(ctx, generation, scope, q, runtime)
	if err != nil {
		return SearchRun{}, &AppError{Kind: ErrFetch, Operation: "start search", Profile: req.Profile.Name, Err: err}
	}
	return SearchRun{Profile: req.Profile, Settings: settings, Scope: scope, Query: q, Filter: req.Filter, Run: run, Generation: generation}, nil
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

func (s Service) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	settings, scope, snapshot, q, err := s.Inventory(ctx, req)
	if err != nil {
		return SearchResult{}, err
	}
	method := target.ResolvedUnique
	if req.Target == "" {
		method = target.ResolvedManual
	}
	resolved, err := ResolveSnapshot(req.Profile, snapshot, q, req.Filter, nil, method)
	if err != nil {
		return SearchResult{Profile: req.Profile, Settings: settings, Scope: scope, Snapshot: snapshot, Query: q, Filter: req.Filter}, classifyResolveError(err, snapshot, q, req.Filter)
	}
	return SearchResult{Profile: req.Profile, Settings: settings, Scope: scope, Snapshot: snapshot, Target: resolved, Query: q, Filter: req.Filter}, nil
}

func classifyResolveError(err error, snapshot inventory.InventorySnapshot, query target.TargetQuery, filter string) error {
	kind := ErrNoTarget
	if !snapshot.EC2Complete || !snapshot.Finished {
		kind = ErrIncomplete
	}
	if err != nil && len(Candidates(snapshot, query, filter)) > 1 {
		kind = ErrAmbiguous
	}
	return &AppError{Kind: kind, Operation: "resolve target", Err: err}
}
