package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

// ssmm と AWS に異なる名前を使い、各 CLI 経路が検索・接続には AWS プロファイル、SSH の既定値には ssmm プロファイルを使うことを検証する。
func TestCommandsSeparateSSMMAndAWSProfiles(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	for _, selection := range []struct {
		name, configured, environment, override string
		want                                    target.ProfileSelection
	}{
		{"configured", "company-prod", "unrelated", "", target.ProfileSelection{Name: "company-prod", Source: target.ProfileConfig}},
		{"environment", "", "company-env", "", target.ProfileSelection{Name: "company-env", Source: target.ProfileEnv}},
		{"default", "", "", "", target.ProfileSelection{Name: "default", Source: target.ProfileDefault}},
		{"override", "saved-account", "environment-account", "command-account", target.ProfileSelection{Name: "command-account", Source: target.ProfileFlag}},
	} {
		t.Run(selection.name, func(t *testing.T) {
			t.Setenv("AWS_PROFILE", selection.environment)
			store, _ := sshConfigTestStore(t)
			command := New(Dependencies{Store: store, Output: &bytes.Buffer{}})
			region := "us-east-1"
			if selection.override != "" {
				region = "us-west-2"
			}
			command.SetArgs([]string{"init", "--ssh", "-s", "web-prod", "--profile", selection.configured, "--regions", region, "--user", "ubuntu", "--identity-file", "/keys/web.pem", "--port", "2222"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, route := range []struct {
				args        []string
				search, ssh bool
			}{
				{[]string{"list", "-s", "web-prod"}, true, false},
				{[]string{"connect", "web", "-s", "web-prod"}, true, false},
				{[]string{"ssh", "web", "-s", "web-prod"}, true, true},
				{[]string{"scp", "file", "web:/tmp/file", "-s", "web-prod"}, true, true},
				{[]string{"proxy", "web.web-prod.ssmm"}, true, false},
				{[]string{"proxy", "web", "-s", "web-prod"}, true, false},
				{[]string{"proxy", "i-01234567", "-s", "web-prod", "--region", "us-east-1"}, false, false},
			} {
				t.Run(strings.Join(route.args, " "), func(t *testing.T) {
					backend := &profileBackend{}
					planner := &profilePlanner{}
					var output bytes.Buffer
					service := app.Service{Settings: store, Profiles: backend, AWS: backend, Session: planner, SSH: planner}
					command := New(Dependencies{Service: service, Profiles: backend, Runner: planner, Output: &output, Error: &output})
					args := append([]string(nil), route.args...)
					if selection.override != "" {
						args = append(args, "-p", selection.override, "-r", "us-east-1")
					}
					command.SetArgs(args)
					if err := command.Execute(); err != nil {
						t.Fatal(err)
					}
					if len(backend.checked) == 0 {
						t.Fatal("AWS profile was not validated")
					}
					for _, got := range backend.checked {
						if got != selection.want {
							t.Fatalf("validated profile = %#v, want %#v", got, selection.want)
						}
					}
					if route.search && (backend.opened != selection.want || backend.discoveryRegion != "us-east-1") {
						t.Fatalf("AWS search used profile %#v, region %q", backend.opened, backend.discoveryRegion)
					}
					if !route.search && backend.opened.Name != "" {
						t.Fatal("direct proxy opened an AWS search")
					}
					if route.args[0] == "list" {
						if !strings.Contains(output.String(), "Profile: web-prod") {
							t.Fatalf("list did not display ssmm profile: %s", &output)
						}
					} else if planner.resolved.Profile != selection.want || !planner.ran {
						t.Fatalf("connection used %#v, want %#v", planner.resolved.Profile, selection.want)
					}
					if route.ssh && planner.options != (app.SSHOptions{User: "ubuntu", IdentityFile: "/keys/web.pem", Port: 2222}) {
						t.Fatalf("SSH defaults did not use ssmm profile: %#v", planner.options)
					}
				})
			}
		})
	}
}

// 内部 proxy は解決済みの AWS プロファイルを受け取り、ssmm 設定の再読み込みを行わない。
func TestInternalProxyUsesConfiguredAWSProfileDirectly(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("AWS_PROFILE", "")
	backend := &profileBackend{}
	planner := &profilePlanner{}
	command := New(Dependencies{Profiles: backend, Service: app.Service{Session: planner}, Runner: planner})
	command.SetArgs([]string{"proxy", "i-01234567", "--region", "us-east-1", "--profile", "company-prod", "--internal-profile-source", "config"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if planner.resolved.Profile != (target.ProfileSelection{Name: "company-prod", Source: target.ProfileConfig}) || !planner.ran {
		t.Fatalf("unexpected internal proxy target: %#v", planner.resolved)
	}
}

type profileBackend struct {
	checked         []target.ProfileSelection
	opened          target.ProfileSelection
	discoveryRegion string
	query           target.TargetQuery
}

func (b *profileBackend) Validate(_ context.Context, p target.ProfileSelection) error {
	b.checked = append(b.checked, p)
	return p.Validate()
}
func (b *profileBackend) Open(_ context.Context, p target.ProfileSelection) (app.AWSRuntime, error) {
	b.opened = p
	return b, nil
}
func (b *profileBackend) EnabledRegions(_ context.Context, region string) ([]string, error) {
	b.discoveryRegion = region
	return []string{"us-east-1"}, nil
}
func (b *profileBackend) EC2(_ context.Context, region string, query target.TargetQuery, emit func([]target.Instance) error) error {
	b.query = query
	name := "web"
	return emit([]target.Instance{{Key: target.InstanceKey{Region: region, InstanceID: "i-01234567"}, Name: &name, EC2State: "running", Tags: map[string]string{"Environment": "production", "Service": "web"}}})
}
func (b *profileBackend) SSM(context.Context, string, func([]inventory.SSMRecord) error) error {
	return nil
}

type profilePlanner struct {
	resolved  target.ResolvedTarget
	options   app.SSHOptions
	ran       bool
	recursive bool
}

func (p *profilePlanner) PlanSession(req app.SessionRequest) (execplan.ProcessSpec, error) {
	p.resolved = req.Target
	return execplan.ProcessSpec{}, nil
}
func (p *profilePlanner) PlanSSH(req app.SSHRequest) (execplan.ProcessSpec, error) {
	p.resolved, p.options = req.Target, req.Options
	return execplan.ProcessSpec{}, nil
}
func (p *profilePlanner) PlanSCP(req app.SCPRequest) (execplan.ProcessSpec, error) {
	p.recursive = req.Recursive
	p.resolved, p.options = req.Target, req.Options
	return execplan.ProcessSpec{}, nil
}
func (p *profilePlanner) Run(context.Context, execplan.ProcessSpec, execplan.Streams) (execplan.ProcessResult, error) {
	p.ran = true
	return execplan.ProcessResult{}, nil
}

// init を行わずに短縮フラグだけで検索・接続し、ssmm 設定が欠損・不正・保存済みのいずれでも読み込まれないことを検証する。
func TestCommandsWithoutSSMMProfile(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("AWS_PROFILE", "environment-account")
	for _, settings := range []struct{ name, contents string }{
		{"missing", ""},
		{"invalid", "not valid TOML [[["},
		{"default", "[profiles.default]\naws_profile = 'saved-account'\nregions = ['us-west-2']\n[profiles.default.ssh]\nuser = 'saved-user'\n"},
	} {
		t.Run(settings.name, func(t *testing.T) {
			store, paths := sshConfigTestStore(t)
			if settings.contents != "" {
				if err := os.WriteFile(paths.ConfigFile, []byte(settings.contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, route := range []struct {
				args   []string
				search bool
			}{
				{[]string{"web"}, true},
				{[]string{"list"}, true},
				{[]string{"connect", "web"}, true},
				{[]string{"ssh", "web"}, true},
				{[]string{"scp", "file", "web:/tmp/file", "--recursive"}, true},
				{[]string{"proxy", "web"}, true},
				{[]string{"proxy", "i-01234567"}, false},
			} {
				t.Run(strings.Join(route.args, " "), func(t *testing.T) {
					backend := &profileBackend{}
					planner := &profilePlanner{}
					var output bytes.Buffer
					service := app.Service{Settings: store, Profiles: backend, AWS: backend, Session: planner, SSH: planner}
					command := New(Dependencies{Service: service, Profiles: backend, Runner: planner, Output: &output, Error: &output})
					args := append(append([]string(nil), route.args...), "-p", "command-account", "-r", "us-east-1")
					if route.search {
						args = append(args, "-t", "Environment=production", "-t", "Service=web")
					}
					command.SetArgs(args)
					if err := command.Execute(); err != nil {
						t.Fatal(err)
					}
					want := target.ProfileSelection{Name: "command-account", Source: target.ProfileFlag}
					for _, got := range backend.checked {
						if got != want {
							t.Fatalf("profile = %#v, want %#v", got, want)
						}
					}
					if route.search && (backend.opened != want || backend.discoveryRegion != "us-east-1" || len(backend.query.Tags) != 2) {
						t.Fatalf("short flags were not used: %#v", backend)
					}
					if route.args[0] == "list" {
						if !strings.Contains(output.String(), "Profile: (none)") {
							t.Fatalf("unexpected list profile: %s", &output)
						}
					} else if !planner.ran || planner.resolved.Profile != want || planner.options != (app.SSHOptions{}) {
						t.Fatalf("unexpected connection settings: %#v", planner)
					}
					if route.args[0] == "scp" && !planner.recursive {
						t.Fatal("--recursive was not passed to SCP")
					}
				})
			}
			if settings.contents == "" {
				if _, err := os.Stat(paths.ConfigFile); !os.IsNotExist(err) {
					t.Fatalf("command created ssmm config: %v", err)
				}
			} else if got := readSSHConfigTestFile(t, paths.ConfigFile); got != settings.contents {
				t.Fatalf("command modified ssmm config: %s", got)
			}
		})
	}
}

// 明示した ssmm プロファイルの欠損・空指定と、ホスト名との矛盾を AWS 操作の前に拒否する。
func TestCommandsRejectInvalidSSMMProfileSelection(t *testing.T) {
	for _, args := range [][]string{
		{"list", "-s", "missing"},
		{"list", "--ssmm-profile", ""},
		{"proxy", "i-01234567", "-s", "missing", "-r", "us-east-1"},
		{"proxy", "web.prod.ssmm", "-s", "other"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			store, _ := sshConfigTestStore(t)
			backend := &profileBackend{}
			command := New(Dependencies{Service: app.Service{Settings: store, Profiles: backend, AWS: backend}})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("invalid ssmm profile selection was accepted")
			}
			if len(backend.checked) != 0 || backend.opened.Name != "" {
				t.Fatal("invalid ssmm profile triggered AWS operations")
			}
		})
	}
}

// ssmm 設定ストアを用意せず、AWS プロファイルの環境変数と既定値だけで一覧取得できることを検証する。
func TestListWithoutProfileUsesAWSDefaults(t *testing.T) {
	for _, env := range []string{"environment-account", ""} {
		t.Run("AWS_PROFILE="+env, func(t *testing.T) {
			t.Setenv("AWS_PROFILE", env)
			backend := &profileBackend{}
			command := New(Dependencies{Service: app.Service{Profiles: backend, AWS: backend}, Output: &bytes.Buffer{}})
			command.SetArgs([]string{"list", "-r", "us-east-1"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			want := target.ProfileSelection{Name: "default", Source: target.ProfileDefault}
			if env != "" {
				want = target.ProfileSelection{Name: env, Source: target.ProfileEnv}
			}
			if backend.opened != want {
				t.Fatalf("AWS profile = %#v, want %#v", backend.opened, want)
			}
		})
	}
}
