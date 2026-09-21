package openssh

import (
	"testing"

	"github.com/uncho/ssmm/internal/app"
)

func TestParseTransfer(t *testing.T) {
	transfer, err := ParseTransfer([]string{"./local:name", "user@web:/var/tmp/out"})
	if err != nil {
		t.Fatal(err)
	}
	if transfer.Direction != app.SCPSend || transfer.Target != "web" || transfer.Remote[0].Path != "/var/tmp/out" {
		t.Fatalf("unexpected transfer: %#v", transfer)
	}
	if _, err := ParseTransfer([]string{"one:/a", "two:/b", "./out"}); err == nil {
		t.Fatal("different remote targets should be rejected")
	}
}

func TestProxyCommandQuotesFixedArguments(t *testing.T) {
	command, err := ProxyCommand("/tmp/ssmm path", "i-01234567", "ap-northeast-1", 22, "profile's", "flag")
	if err != nil {
		t.Fatal(err)
	}
	if command == "" || command[0] != '\'' || command[len(command)-1] != '\'' {
		t.Fatalf("unexpected shell command: %s", command)
	}
}
