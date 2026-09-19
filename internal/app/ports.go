package app

import (
	"context"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type SettingsStore interface {
	Read(context.Context) (SettingsSnapshot, error)
	ReadForUpdate(context.Context) (SettingsDraft, error)
	Commit(context.Context, SettingsUpdate) (SaveReport, error)
}

type ProfileChecker interface {
	Validate(context.Context, target.ProfileSelection) error
}

type AWSFactory interface {
	Open(context.Context, target.ProfileSelection) (AWSRuntime, error)
}

type AWSRuntime interface {
	inventory.Source
	EnabledRegions(context.Context, string) ([]string, error)
}

type SelectionView struct {
	Snapshot inventory.InventorySnapshot
	Filter   string
	Mode     SelectionMode
}
type SelectionAction struct {
	Kind       ActionKind
	Generation uint64
	Key        target.InstanceKey
	Filter     string
}
type ActionKind string

const (
	FilterChanged ActionKind = "filter-changed"
	Select        ActionKind = "select"
	Refresh       ActionKind = "refresh"
	Cancel        ActionKind = "cancel"
)

type SelectionMode string

const (
	Explicit         SelectionMode = "explicit"
	AutoWhenComplete SelectionMode = "auto-when-complete"
	ManualOnly       SelectionMode = "manual-only"
	NonInteractive   SelectionMode = "non-interactive"
)

type SelectionUI interface {
	Run(context.Context, <-chan SelectionView, chan<- SelectionAction) error
}
type InitUI interface {
	Edit(context.Context, SettingsDraft) (SettingsUpdate, error)
}
type SessionPlanner interface {
	PlanSession(SessionRequest) (execplan.ProcessSpec, error)
}
type SSHPlanner interface {
	PlanSSH(SSHRequest) (execplan.ProcessSpec, error)
	PlanSCP(SCPRequest) (execplan.ProcessSpec, error)
}
type Runner interface {
	Run(context.Context, execplan.ProcessSpec, execplan.Streams) (execplan.ProcessResult, error)
}
