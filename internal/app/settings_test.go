package app

import (
	"strings"
	"testing"
)

func TestSSHOptionsValidate(t *testing.T) {
	for name, options := range map[string]SSHOptions{
		"empty values": {},
		"quoted path":  {User: "ec2-user", IdentityFile: `/tmp/key with 'quote"#%`, Port: 22},
	} {
		t.Run(name, func(t *testing.T) {
			if err := options.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, options := range map[string]SSHOptions{
		"user whitespace":   {User: "ec2 user"},
		"user delimiter":    {User: "ec2@user"},
		"user option":       {User: "-oProxyCommand"},
		"identity variable": {IdentityFile: `${HOME}/key`},
		"identity newline":  {IdentityFile: "/tmp/key\nname"},
		"port":              {Port: 65536},
	} {
		t.Run(name, func(t *testing.T) {
			if err := options.Validate(); err == nil || !strings.Contains(err.Error(), "SSH") && !strings.Contains(err.Error(), "identity") {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
}
