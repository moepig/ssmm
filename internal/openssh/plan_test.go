package openssh

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
)

func TestPlanSSHUsesFixedTargetAndEscapedIdentityFile(t *testing.T) {
	profile := target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}
	plan, err := PlanSSH(SSHRequest{Target: target.ResolvedTarget{Profile: profile, Region: "us-east-1", InstanceID: "i-01234567", Method: target.ResolvedUnique}, Options: SSHOptions{IdentityFile: "/tmp/key%h", Port: 2222}, Executable: "/usr/bin/ssh", Ssmm: "/tmp/ssmm"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, "\x00")
	if !strings.Contains(joined, "IdentityFile=/tmp/key%%h") || !strings.Contains(joined, "HostKeyAlias=ssmm-us-east-1-i-01234567-2222") {
		t.Fatalf("unexpected plan: %#v", plan.Args)
	}
	if strings.Contains(joined, "-i\x00") {
		t.Fatal("identity file should use -o IdentityFile")
	}
	if plan.Args[len(plan.Args)-1] != "i-01234567" {
		t.Fatalf("ssh destination is missing: %#v", plan.Args)
	}
	checkArgs := append([]string{"-G"}, plan.Args...)
	if output, err := exec.Command(plan.Executable, checkArgs...).CombinedOutput(); err != nil {
		t.Fatalf("OpenSSH rejected generated plan: %v\n%s", err, output)
	}
}

func TestPlanSCPRejectsConflictingExplicitUser(t *testing.T) {
	profile := target.ProfileSelection{Name: "prod", Source: target.ProfileFlag}
	_, err := PlanSCP(SCPRequest{Target: target.ResolvedTarget{Profile: profile, Region: "us-east-1", InstanceID: "i-01234567", Method: target.ResolvedUnique}, Options: SSHOptions{User: "ubuntu"}, Executable: "/usr/bin/scp", Ssmm: "/tmp/ssmm", Transfer: Transfer{Direction: Send, Local: []string{"file"}, Remote: []Endpoint{{User: "ec2-user", Target: "web", Path: "/tmp"}}, User: "ec2-user", Target: "web"}})
	if err == nil {
		t.Fatal("conflicting USER@ target should be rejected")
	}
}

// SSH と SCP の内部 proxy に設定由来の AWS プロファイルと指定元が渡ることを検証する。
func TestPlansPassConfiguredAWSProfileToProxy(t *testing.T) {
	resolved := target.ResolvedTarget{Profile: target.ProfileSelection{Name: "company-prod", Source: target.ProfileConfig}, Region: "us-east-1", InstanceID: "i-01234567", Method: target.ResolvedUnique}
	ssh, err := PlanSSH(SSHRequest{Target: resolved, Executable: "/usr/bin/ssh", Ssmm: "/usr/bin/ssmm"})
	if err != nil {
		t.Fatal(err)
	}
	scp, err := PlanSCP(SCPRequest{Target: resolved, Executable: "/usr/bin/scp", Ssmm: "/usr/bin/ssmm", Transfer: Transfer{Direction: Send, Local: []string{"file"}, Remote: []Endpoint{{Target: "web", Path: "/tmp/file"}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []execplan.ProcessSpec{ssh, scp} {
		args := strings.Join(plan.Args, "\x00")
		if !strings.Contains(args, "'--profile' 'company-prod'") || !strings.Contains(args, "'--internal-profile-source' 'config'") || plan.EnvironmentPolicy.Profile != "company-prod" || plan.EnvironmentPolicy.ProfileSource != "config" {
			t.Fatalf("configured AWS profile was not passed: %#v", plan)
		}
	}
}
