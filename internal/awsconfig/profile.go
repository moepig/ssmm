package awsconfig

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	sdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/uncho/ssmm/internal/target"
)

type ProfileChecker struct{}

func (ProfileChecker) Validate(ctx context.Context, selection target.ProfileSelection) error {
	if err := selection.Validate(); err != nil {
		return err
	}
	if selection.Source == target.ProfileDefault {
		return nil
	}
	if selection.Source == target.ProfileEnv && os.Getenv("AWS_PROFILE") != selection.Name {
		return fmt.Errorf("AWS_PROFILE does not match selected profile %q", selection.Name)
	}
	if _, err := sdkconfig.LoadSharedConfigProfile(ctx, selection.Name, sharedProfileFiles()); err != nil {
		return fmt.Errorf("profile %q is not available: %w", selection.Name, err)
	}
	return nil
}

func sharedProfileFiles() func(*sdkconfig.LoadSharedConfigOptions) {
	return func(options *sdkconfig.LoadSharedConfigOptions) {
		if path := strings.TrimSpace(os.Getenv("AWS_CONFIG_FILE")); path != "" {
			options.ConfigFiles = []string{path}
		}
		if path := strings.TrimSpace(os.Getenv("AWS_SHARED_CREDENTIALS_FILE")); path != "" {
			options.CredentialsFiles = []string{path}
		}
	}
}

type Factory struct {
	Checker  ProfileChecker
	Endpoint string
}

var sdkEnvironmentMu sync.Mutex

func (f Factory) Validate(ctx context.Context, selection target.ProfileSelection) error {
	return f.Checker.Validate(ctx, selection)
}

func (f Factory) Open(ctx context.Context, selection target.ProfileSelection) (*Runtime, error) {
	return f.OpenRegion(ctx, selection, "")
}

func (f Factory) OpenRegion(ctx context.Context, selection target.ProfileSelection, region string) (*Runtime, error) {
	if err := f.Validate(ctx, selection); err != nil {
		return nil, err
	}
	cfg, err := loadSDKConfig(ctx, selection, region)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &Runtime{cfg: cfg, selection: selection, endpoint: f.endpoint(), clients: make(map[string]Clients)}, nil
}

func loadSDKConfig(ctx context.Context, selection target.ProfileSelection, region string) (awssdk.Config, error) {
	options := make([]func(*sdkconfig.LoadOptions) error, 0, 2)
	switch selection.Source {
	case target.ProfileFlag, target.ProfileHost:
		options = append(options, sdkconfig.WithSharedConfigProfile(selection.Name))
	case target.ProfileEnv:
		// AWS_PROFILE remains the SDK and CLI profile selector.
	case target.ProfileDefault:
		// The standard provider chain must remain available for environment,
		// container, and instance-role credentials.
	}
	if region != "" {
		options = append(options, sdkconfig.WithRegion(region))
	}

	// AWS_DEFAULT_PROFILE is interpreted by the SDK but is deliberately not
	// part of ssmm's profile precedence. LoadDefaultConfig has no environment
	// slice parameter, so serialize this short read and restore the process
	// environment immediately afterwards.
	sdkEnvironmentMu.Lock()
	defer sdkEnvironmentMu.Unlock()
	type savedEnv struct {
		name  string
		value string
		found bool
	}
	saved := []savedEnv{{name: "AWS_DEFAULT_PROFILE"}}
	if selection.Source == target.ProfileDefault {
		saved = append(saved, savedEnv{name: "AWS_PROFILE"})
	}
	for i := range saved {
		saved[i].value, saved[i].found = os.LookupEnv(saved[i].name)
		_ = os.Unsetenv(saved[i].name)
	}
	defer func() {
		for _, item := range saved {
			if item.found {
				_ = os.Setenv(item.name, item.value)
			} else {
				_ = os.Unsetenv(item.name)
			}
		}
	}()
	return sdkconfig.LoadDefaultConfig(ctx, options...)
}

func (f Factory) endpoint() string {
	if value := strings.TrimSpace(f.Endpoint); value != "" {
		return value
	}
	for _, name := range []string{"SSMM_AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func DiscoveryRegion(cfg awssdk.Config) string {
	if cfg.Region != "" {
		return cfg.Region
	}
	return "us-east-1"
}
