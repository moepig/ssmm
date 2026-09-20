//go:build unix

package process

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
)

func TestEnvironmentNormalizesAWSProfileSelection(t *testing.T) {
	t.Setenv("AWS_PROFILE", "other")
	t.Setenv("AWS_DEFAULT_PROFILE", "default-from-environment")

	value := func(policy execplan.EnvironmentPolicy, name string) string {
		for _, item := range environment(policy) {
			if len(item) > len(name) && item[:len(name)+1] == name+"=" {
				return item[len(name)+1:]
			}
		}
		return ""
	}
	if got := value(execplan.EnvironmentPolicy{Profile: "prod", ProfileSource: string(target.ProfileFlag)}, "AWS_PROFILE"); got != "" {
		t.Fatalf("explicit profile inherited AWS_PROFILE=%q", got)
	}
	if got := value(execplan.EnvironmentPolicy{Profile: "prod", ProfileSource: string(target.ProfileConfig)}, "AWS_PROFILE"); got != "" {
		t.Fatalf("configured profile inherited AWS_PROFILE=%q", got)
	}
	if got := value(execplan.EnvironmentPolicy{Profile: "default", ProfileSource: string(target.ProfileDefault)}, "AWS_PROFILE"); got != "" {
		t.Fatalf("default profile inherited AWS_PROFILE=%q", got)
	}
	if got := value(execplan.EnvironmentPolicy{Profile: "other", ProfileSource: string(target.ProfileEnv)}, "AWS_PROFILE"); got != "other" {
		t.Fatalf("environment profile = %q, want other", got)
	}
	for _, item := range environment(execplan.EnvironmentPolicy{}) {
		if len(item) >= len("AWS_DEFAULT_PROFILE=") && item[:len("AWS_DEFAULT_PROFILE=")] == "AWS_DEFAULT_PROFILE=" {
			t.Fatal("AWS_DEFAULT_PROFILE was passed to child process")
		}
	}
}

func TestRunnerPassesNormalizedProfileEnvironmentToChild(t *testing.T) {
	t.Setenv("AWS_PROFILE", "other")
	t.Setenv("AWS_DEFAULT_PROFILE", "unexpected")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	result, err := (NewRunner()).Run(context.Background(), execplan.ProcessSpec{
		Executable:        "/bin/sh",
		Args:              []string{"-c", "printf '%s|%s' \"$AWS_PROFILE\" \"$AWS_DEFAULT_PROFILE\""},
		Mode:              execplan.ProxyStream,
		EnvironmentPolicy: execplan.EnvironmentPolicy{Profile: "prod", ProfileSource: string(target.ProfileFlag)},
	}, execplan.Streams{Stdout: writer})
	_ = writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || string(output) != "|" {
		t.Fatalf("child received result=%#v environment=%q", result, output)
	}
}
