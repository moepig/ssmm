package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

// 明示 TARGET は候補数が 1 件のときだけ接続し、検索が不完全な場合も拒否する。
func TestExplicitTargetRequiresOneCompleteCandidate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		instances []target.Instance
		ec2Err    error
		wantErr   string
	}{
		{name: "one candidate", instances: []target.Instance{selectionInstance("web")}},
		{name: "no candidates", instances: []target.Instance{selectionInstance("other")}, wantErr: "no matching instance"},
		{name: "multiple candidates", instances: []target.Instance{selectionInstanceWithID("web", "i-00000001"), selectionInstanceWithID("web", "i-00000002")}, wantErr: "target is ambiguous: 2 instances match"},
		{name: "incomplete EC2 inventory", instances: []target.Instance{selectionInstance("web")}, ec2Err: errors.New("EC2 unavailable"), wantErr: "search result is incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSelectionExecutables(t)
			backend := &selectionBackend{instances: tc.instances, ec2Err: tc.ec2Err}
			planner := &selectionPlanner{}
			var output bytes.Buffer
			command := New(Dependencies{
				Service:  app.Service{Profiles: backend, AWS: backend, Session: planner},
				Profiles: backend, Runner: planner, Output: &output, Error: &output,
			})
			command.SetArgs([]string{"web"})
			err := command.Execute()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Execute() error = %v, want containing %q", err, tc.wantErr)
				}
				if planner.ran {
					t.Fatal("connection ran without one complete candidate")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !planner.ran || planner.resolved.InstanceID != "i-00000001" {
				t.Fatalf("connection target = %#v, ran = %t", planner.resolved, planner.ran)
			}
		})
	}
}

type selectionBackend struct {
	instances []target.Instance
	ec2Err    error
}

func (b *selectionBackend) Validate(_ context.Context, profile target.ProfileSelection) error {
	return profile.Validate()
}

func (b *selectionBackend) Open(context.Context, target.ProfileSelection) (app.AWSRuntime, error) {
	return b, nil
}

func (b *selectionBackend) EnabledRegions(context.Context, string) ([]string, error) {
	return []string{"us-east-1"}, nil
}

func (b *selectionBackend) EC2(_ context.Context, _ string, _ target.TargetQuery, emit func([]target.Instance) error) error {
	if err := emit(b.instances); err != nil {
		return err
	}
	return b.ec2Err
}

func (b *selectionBackend) SSM(context.Context, string, func([]inventory.SSMRecord) error) error {
	return nil
}

type selectionPlanner struct {
	resolved target.ResolvedTarget
	ran      bool
}

func (p *selectionPlanner) PlanSession(request app.SessionRequest) (execplan.ProcessSpec, error) {
	p.resolved = request.Target
	return execplan.ProcessSpec{}, nil
}

func (p *selectionPlanner) Run(context.Context, execplan.ProcessSpec, execplan.Streams) (execplan.ProcessResult, error) {
	p.ran = true
	return execplan.ProcessResult{}, nil
}

func selectionInstance(name string) target.Instance {
	return selectionInstanceWithID(name, "i-00000001")
}

func selectionInstanceWithID(name, id string) target.Instance {
	return target.Instance{
		Key:      target.InstanceKey{Region: "us-east-1", InstanceID: id},
		Name:     &name,
		EC2State: "running",
	}
}

func setupSelectionExecutables(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}
