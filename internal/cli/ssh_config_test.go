package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/config"
)

// 一時ディレクトリで作成・再作成・削除を実行し、既存設定の保持と最後のプロファイル削除時の Include 解除を検証する。
func TestSSHConfigCreateDeletePreservesSettings(t *testing.T) {
	store, paths := sshConfigTestStore(t)
	existing := "# Personal settings\nHost personal\n    HostName example.com\n"
	if err := os.MkdirAll(filepath.Dir(paths.SSHConfigFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHConfigFile, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		var output bytes.Buffer
		command := New(Dependencies{Store: store, Output: &output})
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		savedPaths := []string{paths.ConfigFile}
		if args[0] == "ssh-config" {
			savedPaths = append(savedPaths, paths.SSHManagedFile, paths.SSHConfigFile)
		}
		for _, path := range savedPaths {
			if !strings.Contains(output.String(), path+": saved") {
				t.Fatalf("missing save report for %s: %s", path, &output)
			}
		}
	}
	run("init", "--ssh", "-s", "prod", "--regions", "ap-northeast-1", "--user", "ec2-user", "--identity-file", "keys/prod.pem", "--port", "2222")
	initial, err := store.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if initial.Profile("prod").Integration {
		t.Fatal("init enabled SSH config")
	}
	for _, step := range []struct {
		args []string
		prod bool
		dev  bool
	}{
		{[]string{"ssh-config", "create", "-s", "prod"}, true, false},
		{[]string{"ssh-config", "create", "-s", "prod"}, true, false},
		{[]string{"ssh-config", "-s", "dev", "create"}, true, true},
		{[]string{"init", "--ssh", "-s", "prod", "--port", "2222"}, true, true},
		{[]string{"ssh-config", "delete", "-s", "prod"}, false, true},
		{[]string{"ssh-config", "delete", "-s", "prod"}, false, true},
		{[]string{"ssh-config", "delete", "-s", "dev"}, false, false},
		{[]string{"init", "--ssh", "-s", "prod", "--port", "2222"}, false, false},
	} {
		run(step.args...)
		snapshot, err := store.Read(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		prod := snapshot.Profile("prod")
		if prod.SSH != initial.Profile("prod").SSH || prod.Regions == nil || strings.Join(*prod.Regions, ",") != "ap-northeast-1" {
			t.Fatalf("%v changed profile settings: %#v", step.args, prod)
		}
		managed := readSSHConfigTestFile(t, paths.SSHManagedFile)
		for name, wanted := range map[string]bool{"prod": step.prod, "dev": step.dev} {
			if snapshot.Profile(name).Integration != wanted || strings.Contains(managed, "Host *."+name+".ssmm") != wanted {
				t.Fatalf("%v: unexpected SSH config for %s: %s", step.args, name, managed)
			}
		}
		ssh := readSSHConfigTestFile(t, paths.SSHConfigFile)
		includeCount := 0
		if step.prod || step.dev {
			includeCount = 1
		}
		if strings.Count(ssh, "Include "+paths.SSHManagedFile) != includeCount || !strings.HasSuffix(ssh, existing) {
			t.Fatalf("%v changed personal settings or Include count: %s", step.args, ssh)
		}
		if includeCount == 0 && ssh != existing {
			t.Fatalf("SSH config was not restored: %q", ssh)
		}
	}
}

// AWS_PROFILE の有無にかかわらず、無指定時は ssmm の default プロファイルに保存することを検証する。
func TestSSHConfigProfileDefaults(t *testing.T) {
	for _, env := range []string{"staging", ""} {
		t.Run("AWS_PROFILE="+env, func(t *testing.T) {
			t.Setenv("AWS_PROFILE", env)
			store, _ := sshConfigTestStore(t)
			command := New(Dependencies{Store: store, Output: &bytes.Buffer{}})
			command.SetArgs([]string{"ssh-config", "create"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			name := "default"
			if len(snapshot.Profiles) != 1 || !snapshot.Profile(name).Integration {
				t.Fatalf("unexpected profiles: %#v", snapshot.Profiles)
			}
		})
	}
}

// 廃止フラグと不正な操作・プロファイル名を拒否し、設定ファイルが作成されないことを検証する。
func TestSSHConfigRejectsInvalidInputBeforeSaving(t *testing.T) {
	for _, args := range [][]string{
		{"init", "--integration"},
		{"init", "--integration=false"},
		{"ssh-config", "create", "-s", "Prod_Profile"},
		{"ssh-config", "create", "unexpected"},
		{"ssh-config", "delete", "unexpected"},
		{"ssh-config", "unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			store, paths := sshConfigTestStore(t)
			command := New(Dependencies{Store: store, Output: &bytes.Buffer{}})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("invalid input was accepted")
			}
			for _, path := range []string{paths.ConfigFile, paths.SSHManagedFile, paths.SSHConfigFile} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected file state for %s: %v", path, err)
				}
			}
		})
	}
}

func sshConfigTestStore(t *testing.T) (config.AppStore, config.Paths) {
	t.Helper()
	root := t.TempDir()
	paths := config.Paths{
		ConfigFile:     filepath.Join(root, "config.toml"),
		SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"),
		SSHConfigFile:  filepath.Join(root, "ssh", "config"),
		LockFile:       filepath.Join(root, "lock"),
	}
	return config.NewAppStore(paths, "/usr/local/bin/ssmm"), paths
}

func readSSHConfigTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
