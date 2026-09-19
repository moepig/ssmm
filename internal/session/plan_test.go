package session

import (
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/target"
)

func TestPlanProxyUsesJSONPortParameter(t *testing.T) {
	plan, err := Plan(Request{AWS: "/usr/bin/aws", Proxy: true, Port: 2222, Target: target.ResolvedTarget{Profile: target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}, Region: "us-east-1", InstanceID: "i-01234567", Method: target.ResolvedDirect}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, "\x00")
	if !strings.Contains(joined, `{"portNumber":["2222"]}`) {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
	if !strings.Contains(joined, "--profile\x00prod") {
		t.Fatalf("explicit profile was not passed: %#v", plan.Args)
	}
}
