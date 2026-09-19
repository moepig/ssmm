package awsconfig

import (
	"context"
	"os"
	"strings"

	"github.com/uncho/ssmm/internal/target"
)

func SelectionFromFlags(name string, explicitlySet bool) target.ProfileSelection {
	if explicitlySet {
		return target.ProfileSelection{Name: name, Source: target.ProfileFlag}
	}
	if env := os.Getenv("AWS_PROFILE"); env != "" {
		return target.ProfileSelection{Name: env, Source: target.ProfileEnv}
	}
	return target.ProfileSelection{Name: "default", Source: target.ProfileDefault}
}

func ConfiguredRegion(ctx context.Context, selection target.ProfileSelection) string {
	cfg, err := loadSDKConfig(ctx, selection, "")
	if err == nil && cfg.Region != "" {
		return cfg.Region
	}
	if value := os.Getenv("AWS_REGION"); value != "" {
		return value
	}
	if value := os.Getenv("AWS_DEFAULT_REGION"); value != "" {
		return value
	}
	return "us-east-1"
}

func NormalizeEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, item := range env {
		if strings.HasPrefix(item, "AWS_DEFAULT_PROFILE=") {
			continue
		}
		out = append(out, item)
	}
	return out
}
