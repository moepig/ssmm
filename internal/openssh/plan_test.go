package openssh

import (
	"os/exec"
	"strings"
	"testing"

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
