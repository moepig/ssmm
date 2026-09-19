package config

import "testing"

func TestDecodeDistinguishesMissingAndEmptyRegions(t *testing.T) {
	settings, err := Decode([]byte("[profiles.dev]\n"), "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if settings.Profiles["dev"].Regions != nil {
		t.Fatal("missing regions must mean all regions")
	}
	_, err = Decode([]byte("[profiles.dev]\nregions = []\n"), "/tmp")
	if err == nil {
		t.Fatal("empty regions must be rejected")
	}
}

func TestDecodeRejectsUnknownKey(t *testing.T) {
	if _, err := Decode([]byte("[profiles.dev]\nunknown = true\n"), "/tmp"); err == nil {
		t.Fatal("unknown key was accepted")
	}
}
