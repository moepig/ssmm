package target

import "testing"

func TestValidateSSMMProfileName(t *testing.T) {
	for _, name := range []string{"prod", "Prod_Profile", "prod.example", "prod+ops", "prod@ops"} {
		if err := ValidateSSMMProfileName(name); err != nil {
			t.Errorf("%q rejected: %v", name, err)
		}
	}
	for _, name := range []string{"", "prod!", "prod name", "prod\nname", "-prod"} {
		if err := ValidateSSMMProfileName(name); err == nil {
			t.Errorf("%q accepted", name)
		}
	}
}

func TestValidateSSHLabel(t *testing.T) {
	if err := ValidateSSHLabel("a"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSSHLabel("a" + "bcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuv"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"", "A", "a_b", "-a", "a-", "a.b"} {
		if err := ValidateSSHLabel(label); err == nil {
			t.Errorf("%q accepted", label)
		}
	}
}
