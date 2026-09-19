package cli

import (
	"strings"
	"testing"
)

func TestProfileShorthandIsAvailableOnDocumentedCommands(t *testing.T) {
	root := New(Dependencies{})
	for _, name := range []string{"list", "connect", "ssh", "scp", "proxy", "init"} {
		command, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		flag := command.Flags().ShorthandLookup("p")
		if flag == nil || flag.Name != "profile" {
			t.Fatalf("%s does not expose -p for --profile", name)
		}
	}
}

func TestDocumentedProfileShorthandPassesThroughCobra(t *testing.T) {
	for _, args := range [][]string{
		{"list", "-p", "prod"},
		{"connect", "-p", "prod"},
		{"ssh", "-p", "prod"},
		{"scp", "-p", "prod", "local", "web:/tmp"},
		{"proxy", "i-01234567", "-p", "prod"},
		{"init", "-p", "prod"},
	} {
		command := New(Dependencies{})
		command.SetArgs(args)
		err := command.Execute()
		if err != nil && strings.Contains(err.Error(), "unknown shorthand flag") {
			t.Fatalf("Cobra rejected -p for %v: %v", args, err)
		}
	}
}
