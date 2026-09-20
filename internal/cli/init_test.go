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

// SSH 設定の存在・読み取り可否に依存せず init が成功し、SSH 用ディレクトリを作成・変更しないことを検証する。
func TestInitOnlyWritesSSMMSettings(t *testing.T) {
	for _, sshState := range []string{"absent", "existing", "blocked"} {
		t.Run(sshState, func(t *testing.T) {
			store, paths := sshConfigTestStore(t)
			sshDir := filepath.Dir(paths.SSHConfigFile)
			var before os.FileInfo
			if sshState == "blocked" {
				if err := os.WriteFile(sshDir, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if sshState == "existing" {
				if err := os.MkdirAll(sshDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(paths.SSHConfigFile, []byte("Host personal\n    User me\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				var err error
				before, err = os.Stat(paths.SSHConfigFile)
				if err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			command := New(Dependencies{Store: store, Output: &output})
			command.SetArgs([]string{"init", "-s", "web-prod", "--profile", "company-prod", "--regions", "ap-northeast-1"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Profile("web-prod").AWSProfile != "company-prod" {
				t.Fatalf("AWS profile was not saved: %#v", snapshot)
			}
			if strings.Contains(output.String(), sshDir) {
				t.Fatalf("init reported SSH writes: %s", &output)
			}
			if strings.Contains(readSSHConfigTestFile(t, paths.ConfigFile), ".ssh]") {
				t.Fatal("init created SSH settings without --ssh")
			}
			if sshState == "absent" {
				if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
					t.Fatalf("init created SSH directory: %v", err)
				}
			}
			if before != nil {
				after, err := os.Stat(paths.SSHConfigFile)
				if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
					t.Fatalf("init modified SSH config: %v", err)
				}
			}
		})
	}
}

// SSH 用フラグは --ssh を伴う場合に限って受け付ける。
func TestInitSSHFlagsRequireOptIn(t *testing.T) {
	for _, flag := range []struct{ name, value string }{{"--user", "ubuntu"}, {"--identity-file", "key.pem"}, {"--port", "2222"}} {
		t.Run(flag.name, func(t *testing.T) {
			store, paths := sshConfigTestStore(t)
			for _, prefix := range [][]string{{"init"}, {"init", "--ssh=false"}} {
				command := New(Dependencies{Store: store, Output: &bytes.Buffer{}})
				command.SetArgs(append(prefix, flag.name, flag.value))
				if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "require --ssh") {
					t.Fatalf("SSH flag accepted without --ssh: %v", err)
				}
			}
			if _, err := os.Stat(paths.ConfigFile); !os.IsNotExist(err) {
				t.Fatalf("invalid init saved config: %v", err)
			}
		})
	}
}

// SSH 連携済みのプロファイルでも init は設定値だけを保存し、create の再実行時に生成ファイルへ反映する。
func TestInitSSHDefersGeneratedConfigUpdateUntilCreate(t *testing.T) {
	store, paths := sshConfigTestStore(t)
	run := func(args ...string) {
		t.Helper()
		command := New(Dependencies{Store: store, Output: &bytes.Buffer{}})
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--ssh", "-s", "web-prod", "--user", "ubuntu")
	if _, err := os.Stat(filepath.Dir(paths.SSHConfigFile)); !os.IsNotExist(err) {
		t.Fatalf("init --ssh created an SSH directory: %v", err)
	}
	run("ssh-config", "create", "-s", "web-prod")
	before := readSSHConfigTestFile(t, paths.SSHManagedFile)
	managedInfo, err := os.Stat(paths.SSHManagedFile)
	if err != nil {
		t.Fatal(err)
	}
	sshInfo, err := os.Stat(paths.SSHConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	run("init", "-s", "web-prod", "--profile", "company-prod")
	snapshot, err := store.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if profile := snapshot.Profile("web-prod"); profile.SSH.User != "ubuntu" || !profile.Integration {
		t.Fatalf("plain init changed SSH settings: %#v", profile)
	}
	run("init", "--ssh", "-s", "web-prod", "--user", "ec2-user")
	for path, info := range map[string]os.FileInfo{paths.SSHManagedFile: managedInfo, paths.SSHConfigFile: sshInfo} {
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) {
			t.Fatalf("init modified %s: %v", path, err)
		}
	}
	if got := readSSHConfigTestFile(t, paths.SSHManagedFile); got != before {
		t.Fatalf("init regenerated SSH config: %s", got)
	}
	run("ssh-config", "create", "-s", "web-prod")
	managed := readSSHConfigTestFile(t, paths.SSHManagedFile)
	if !strings.Contains(managed, "User ec2-user") || !strings.Contains(managed, "Host *.web-prod.ssmm") || strings.Contains(managed, "company-prod") {
		t.Fatalf("create did not use updated SSH settings and ssmm profile name: %s", managed)
	}
}

func TestInitStoresRelativeIdentityFileAgainstInvocationDirectory(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigFile:     filepath.Join(root, "config", "config.toml"),
		SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"),
		SSHConfigFile:  filepath.Join(root, "ssh", "config"),
		LockFile:       filepath.Join(root, "config", "lock"),
	}
	store := config.NewAppStore(paths, "/usr/local/bin/ssmm")
	command := New(Dependencies{Store: store})
	command.SetArgs([]string{"init", "--ssh", "-s", "prod", "--identity-file", "keys/prod.pem"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	wantedWorkingPath, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wanted := filepath.Join(wantedWorkingPath, "keys", "prod.pem")
	snapshot, err := store.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	profile := snapshot.Profile("prod")
	if profile.SSH.IdentityFile != wanted {
		t.Fatalf("reloaded identity file = %q, want %q", profile.SSH.IdentityFile, wanted)
	}
	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || !strings.Contains(string(data), wanted) {
		t.Fatalf("config did not persist the resolved identity file: %s", data)
	}
}
