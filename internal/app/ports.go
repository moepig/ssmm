package app

import (
	"context"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type SettingsStore interface {
	Read(context.Context) (SettingsSnapshot, error)
	ReadForUpdate(ctx context.Context, updateSSH bool) (SettingsDraft, error)
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
