package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

func TestPrintListDefaultsToHumanReadableTable(t *testing.T) {
	name := "web"
	snapshot := inventory.InventorySnapshot{
		Instances: []target.Instance{{
			Key:       target.InstanceKey{Region: "ap-northeast-1", InstanceID: "i-0123456789abcdef0"},
			Name:      &name,
			EC2State:  "running",
			SSMStatus: target.SSMOnline,
		}},
		Progress: []inventory.RegionProgress{{
			Region: "ap-northeast-1",
			EC2:    inventory.FetchSucceeded,
			SSM:    inventory.FetchSucceeded,
		}},
	}

	var output strings.Builder
	if err := PrintList(&output, snapshot, "default", inventory.SearchScope{Regions: []string{"ap-northeast-1"}, Source: "configured"}, ""); err != nil {
		t.Fatal(err)
	}

	text := output.String()
	for _, want := range []string{"Profile: default", "NAME", "web", "i-0123456789abcdef0", "Online"} {
		if !strings.Contains(text, want) {
			t.Fatalf("human-readable list output does not contain %q:\n%s", want, text)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Fatalf("default list output unexpectedly looks like JSON:\n%s", text)
	}
}

func TestPrintListAlignsHeadersAndEscapesValues(t *testing.T) {
	names := []string{"日本語", "a-name-longer-than-the-old-fixed-width", "web\n\x1b[31m"}
	snapshot := inventory.InventorySnapshot{}
	for _, name := range names {
		snapshot.Instances = append(snapshot.Instances, target.Instance{
			Key:  target.InstanceKey{Region: "ap-northeast-1", InstanceID: "i-1"},
			Name: &name, EC2State: "running", SSMStatus: target.SSMOnline,
		})
	}
	var output strings.Builder
	if err := PrintList(&output, snapshot, "default", inventory.SearchScope{}, "table"); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Contains(text, "\x1b") || !strings.Contains(text, `web\u000a\u001b[31m`) {
		t.Fatalf("unsafe table output: %q", text)
	}
	column := -1
	for _, line := range strings.Split(text, "\n") {
		pos := strings.Index(line, "REGION")
		if pos >= 0 {
			column = uniseg.StringWidth(line[:pos])
			continue
		}
		if pos = strings.Index(line, "ap-northeast-1"); pos >= 0 {
			if got := uniseg.StringWidth(line[:pos]); got != column {
				t.Fatalf("region column %d, header %d: %q", got, column, line)
			}
		}
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestPrintListReturnsWriteError(t *testing.T) {
	want := errors.New("output failed")
	if err := PrintList(failingWriter{want}, inventory.InventorySnapshot{}, "", inventory.SearchScope{}, "table"); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
