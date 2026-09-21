package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/app"
)

func TestAppStoreCommitWritesConfigAndSSHIntegration(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigFile: filepath.Join(root, "config.toml"), SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"), SSHConfigFile: filepath.Join(root, "ssh", "config"), LockFile: filepath.Join(root, "lock")}
	store := NewAppStore(paths, "/usr/local/bin/ssmm")
	draft, err := store.ReadForUpdate(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	regions := []string{"us-east-1"}
	draft.Snapshot.Profiles["prod"] = app.ProfileSettings{Regions: &regions, Integration: true, SSH: app.SSHOptions{User: "ec2-user", Port: 22}}
	if _, err := store.Commit(context.Background(), app.SettingsUpdate{UpdateSSH: true, Snapshot: draft.Snapshot, Original: draft.Original}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "us-east-1") {
		t.Fatalf("config was not written: %s", data)
	}
	sshData, err := os.ReadFile(paths.SSHConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sshData), "Include "+paths.SSHManagedFile) {
		t.Fatalf("SSH include was not written: %s", sshData)
	}
	managed, err := os.ReadFile(paths.SSHManagedFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(managed), "Host *.prod.ssmm") {
		t.Fatalf("managed config was not written: %s", managed)
	}
}

func TestAppStoreCommitRejectsInvalidUpdateBeforeChangingFiles(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigFile: filepath.Join(root, "config.toml"), SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"), SSHConfigFile: filepath.Join(root, "ssh", "config"), LockFile: filepath.Join(root, "lock")}
	store := NewAppStore(paths, "/usr/local/bin/ssmm")
	initial := []byte("[profiles.prod]\n[profiles.prod.ssh]\nport = 22\n")
	if err := os.WriteFile(paths.ConfigFile, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.SSHManagedFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHManagedFile, []byte("managed-before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHConfigFile, []byte("Include existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	draft, err := store.ReadForUpdate(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	profile := draft.Snapshot.Profiles["prod"]
	profile.SSH.Port = -1
	draft.Snapshot.Profiles["prod"] = profile
	before := readFiles(t, paths)
	if report, err := store.Commit(context.Background(), app.SettingsUpdate{UpdateSSH: true, Snapshot: draft.Snapshot, Original: draft.Original}); err == nil {
		t.Fatalf("invalid update was accepted, report=%#v", report)
	}
	after := readFiles(t, paths)
	for path, expected := range before {
		if string(after[path]) != string(expected) {
			t.Fatalf("invalid update changed %s: before %q after %q", path, expected, after[path])
		}
	}
}

func TestAppStoreCommitGeneratesBeforeSavingInvalidSSHIntegration(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigFile: filepath.Join(root, "config.toml"), SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"), SSHConfigFile: filepath.Join(root, "ssh", "config"), LockFile: filepath.Join(root, "lock")}
	store := NewAppStore(paths, "/usr/local/bin/ssmm")
	if err := os.WriteFile(paths.ConfigFile, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.SSHManagedFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHManagedFile, []byte("managed-before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHConfigFile, []byte("ssh-before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	draft, err := store.ReadForUpdate(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	draft.Snapshot.Profiles["Prod_Profile"] = app.ProfileSettings{Integration: true}
	before := readFiles(t, paths)
	if report, err := store.Commit(context.Background(), app.SettingsUpdate{UpdateSSH: true, Snapshot: draft.Snapshot, Original: draft.Original}); err == nil {
		t.Fatalf("invalid SSH profile was accepted, report=%#v", report)
	}
	after := readFiles(t, paths)
	for path, expected := range before {
		if string(after[path]) != string(expected) {
			t.Fatalf("invalid SSH integration changed %s: before %q after %q", path, expected, after[path])
		}
	}
}

func TestAppStoreCommitReportsPartialWriteFailure(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigFile: filepath.Join(root, "config.toml"), SSHManagedFile: filepath.Join(root, "ssh", "ssmm", "config"), SSHConfigFile: filepath.Join(root, "ssh", "config"), LockFile: filepath.Join(root, "lock")}
	store := NewAppStore(paths, "/usr/local/bin/ssmm")
	if err := os.MkdirAll(filepath.Dir(paths.SSHManagedFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SSHManagedFile, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	draft, err := store.ReadForUpdate(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(paths.SSHManagedFile)
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0o700)
	report, err := store.Commit(context.Background(), app.SettingsUpdate{UpdateSSH: true, Snapshot: draft.Snapshot, Original: draft.Original})
	if err == nil {
		t.Fatal("write failure was accepted")
	}
	if len(report.Files) != 2 || report.Files[0].Status != "saved" || report.Files[1].Status != "failed" || report.Files[1].Path != paths.SSHManagedFile {
		t.Fatalf("partial write report = %#v", report.Files)
	}
}

func TestReadForUpdateUsesOneConfigStateForDraftAndConflictCheck(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigFile: filepath.Join(root, "config.toml"), LockFile: filepath.Join(root, "lock")}
	store := NewAppStore(paths, "/usr/local/bin/ssmm")
	old := []byte("[profiles.old]\n")
	newData := []byte("[profiles.new]\n")
	if err := os.WriteFile(paths.ConfigFile, old, 0o600); err != nil {
		t.Fatal(err)
	}
	changed := false
	readState := func(path string) (app.FileState, error) {
		state, err := readFileState(path)
		if err != nil {
			return app.FileState{}, err
		}
		if path == paths.ConfigFile && !changed {
			changed = true
			if err := os.WriteFile(path, newData, 0o600); err != nil {
				return app.FileState{}, err
			}
		}
		return state, nil
	}
	draft, err := store.readForUpdate(context.Background(), false, readState)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := draft.Snapshot.Profiles["old"]; !ok {
		t.Fatalf("draft was not decoded from the original state: %#v", draft.Snapshot.Profiles)
	}
	if string(draft.Original[paths.ConfigFile].Data) != string(old) {
		t.Fatalf("original state = %q, want %q", draft.Original[paths.ConfigFile].Data, old)
	}
	if _, err := store.Commit(context.Background(), app.SettingsUpdate{Snapshot: draft.Snapshot, Original: draft.Original}); err == nil {
		t.Fatal("commit accepted a changed config")
	}
	got, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newData) {
		t.Fatalf("changed config was overwritten: %q", got)
	}
}

func readFiles(t *testing.T, paths Paths) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, path := range []string{paths.ConfigFile, paths.SSHManagedFile, paths.SSHConfigFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[path] = data
	}
	return out
}
