package sshconfig

import (
	"strings"
	"testing"
)

func TestEnsureIncludeDeduplicatesOnlyManagedLine(t *testing.T) {
	input := []byte("# keep\nInclude /tmp/ssmm/config\nHost *\nInclude /tmp/ssmm/config\n")
	result := string(EnsureInclude(input, "/tmp/ssmm/config"))
	if result != "# keep\nInclude /tmp/ssmm/config\nHost *\n" {
		t.Fatalf("unexpected include result: %q", result)
	}
}

func TestEnsureIncludeQuotesPathWithSpaces(t *testing.T) {
	result := string(EnsureInclude(nil, "/tmp/with space/ssmm/config"))
	if result != "Include \"/tmp/with space/ssmm/config\"\n" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestRenderManagedKeepsHostnameAndEncodesTokens(t *testing.T) {
	data, err := RenderManaged([]Profile{{Name: "prod", User: "ec2-user", IdentityFile: "/tmp/key%h", Port: 2222, Integration: true}}, "/tmp/ssmm")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, expected := range []string{"Host *.prod.ssmm", "HostName %h", "CanonicalizeHostname no", "IdentityFile /tmp/key%%h", "Port 2222", "ProxyCommand '/tmp/ssmm' proxy '%h' --ssmm-profile 'prod' --port '%p'", "ControlPath none"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("managed config lacks %q: %s", expected, text)
		}
	}
}
