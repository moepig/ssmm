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

// 設定で選択した AWS プロファイルが SSM セッションの CLI 引数と環境ポリシーへ渡ることを検証する。
func TestPlanUsesConfiguredAWSProfile(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		plan, err := Plan(Request{AWS: "/usr/bin/aws", Proxy: proxy, Port: 22, Target: target.ResolvedTarget{Profile: target.ProfileSelection{Name: "company-prod", Source: target.ProfileConfig}, Region: "us-east-1", InstanceID: "i-01234567", Method: target.ResolvedDirect}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(plan.Args, "\x00"), "--profile\x00company-prod") || plan.EnvironmentPolicy.ProfileSource != "config" || plan.EnvironmentPolicy.Profile != "company-prod" {
			t.Fatalf("configured AWS profile was not passed: %#v", plan)
		}
	}
}
