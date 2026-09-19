package awsconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/uncho/ssmm/internal/target"
)

func TestFactoryUsesSelectedProfileWhenDefaultProfileEnvironmentDiffers(t *testing.T) {
	root := t.TempDir()
	credentials := filepath.Join(root, "credentials")
	data := []byte("[default]\naws_access_key_id = DEFAULT\naws_secret_access_key = secret\n\n[prod]\naws_access_key_id = PROD\naws_secret_access_key = secret\n\n[other]\naws_access_key_id = OTHER\naws_secret_access_key = secret\n")
	if err := os.WriteFile(credentials, data, 0o600); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "config")
	if err := os.WriteFile(configFile, []byte("[default]\nregion = \"us-east-1\"\n[profile prod]\nregion = \"us-east-1\"\n[profile other]\nregion = \"us-east-1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentials)
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_DEFAULT_PROFILE", "other")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_PROFILE", "")

	factory := Factory{}
	tests := []struct {
		name      string
		selection target.ProfileSelection
		wantKey   string
	}{
		{name: "default", selection: target.ProfileSelection{Name: "default", Source: target.ProfileDefault}, wantKey: "DEFAULT"},
		{name: "environment", selection: target.ProfileSelection{Name: "prod", Source: target.ProfileEnv}, wantKey: "PROD"},
		{name: "explicit", selection: target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}, wantKey: "PROD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.selection.Source == target.ProfileEnv {
				t.Setenv("AWS_PROFILE", tt.selection.Name)
			} else {
				t.Setenv("AWS_PROFILE", "other")
			}
			runtime, err := factory.Open(context.Background(), tt.selection)
			if err != nil {
				t.Fatal(err)
			}
			credentials, err := runtime.cfg.Credentials.Retrieve(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if credentials.AccessKeyID != tt.wantKey {
				t.Fatalf("selected access key = %q, want %q", credentials.AccessKeyID, tt.wantKey)
			}
		})
	}
}

func TestFactoryDefaultKeepsEnvironmentCredentialChain(t *testing.T) {
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_DEFAULT_PROFILE", "other")
	t.Setenv("AWS_ACCESS_KEY_ID", "ENVIRONMENT")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))

	runtime, err := (Factory{}).Open(context.Background(), target.ProfileSelection{Name: "default", Source: target.ProfileDefault})
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := runtime.cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.AccessKeyID != "ENVIRONMENT" {
		t.Fatalf("default provider chain selected %q, want environment credentials", credentials.AccessKeyID)
	}
}
