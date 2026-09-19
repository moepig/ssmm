package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/config"
)

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
	command.SetArgs([]string{"init", "-p", "prod", "--identity-file", "keys/prod.pem"})
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
