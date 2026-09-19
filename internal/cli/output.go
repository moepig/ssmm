package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/render"
	"github.com/uncho/ssmm/internal/target"
)

type ListRow struct {
	InstanceID       string            `json:"instance_id"`
	Name             *string           `json:"name"`
	Region           string            `json:"region"`
	EC2State         string            `json:"ec2_state"`
	SSMStatus        string            `json:"ssm_status"`
	PrivateIP        *string           `json:"private_ip"`
	AvailabilityZone *string           `json:"availability_zone"`
	Tags             map[string]string `json:"tags"`
}

const defaultListFormat = "table"

func Rows(snapshot inventory.InventorySnapshot) []ListRow {
	rows := make([]ListRow, 0, len(snapshot.Instances))
	for _, instance := range target.Sort(snapshot.Instances) {
		tags := instance.Tags
		if tags == nil {
			tags = map[string]string{}
		}
		rows = append(rows, ListRow{InstanceID: instance.Key.InstanceID, Name: instance.Name, Region: instance.Key.Region, EC2State: instance.EC2State, SSMStatus: string(instance.SSMStatus), PrivateIP: instance.PrivateIP, AvailabilityZone: instance.AvailabilityZone, Tags: tags})
	}
	return rows
}

func PrintList(w io.Writer, snapshot inventory.InventorySnapshot, profile string, scope inventory.SearchScope, format string) error {
	if format == "" {
		format = defaultListFormat
	}
	rows := Rows(snapshot)
	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(rows)
	case "id":
		for _, row := range rows {
			if _, err := fmt.Fprintln(w, row.InstanceID); err != nil {
				return err
			}
		}
		return nil
	case "table":
		if _, err := fmt.Fprintln(w, render.Line(0, 4, "Profile: "+profile, fmt.Sprintf("Regions: %d / %d completed", completedRegions(snapshot), len(scope.Regions)))); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, render.Line(0, 4, "Scope: "+strings.Join(scope.Regions, ", "), "Source: "+scope.Source)+"\n"); err != nil {
			return err
		}
		table := render.Table{Indent: 2, Gap: 3, Rows: [][]string{{"NAME", "REGION", "INSTANCE ID", "STATE", "SSM"}}}
		for _, row := range rows {
			name := "-"
			if row.Name != nil {
				name = *row.Name
			}
			table.Rows = append(table.Rows, []string{name, row.Region, row.InstanceID, row.EC2State, row.SSMStatus})
		}
		_, err := io.WriteString(w, table.String())
		return err
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func PrintDiagnostics(w io.Writer, snapshot inventory.InventorySnapshot) {
	for _, progress := range snapshot.Progress {
		if progress.EC2Error != "" {
			fmt.Fprintf(w, "region %s EC2: %s\n", progress.Region, progress.EC2Error)
		}
		if progress.SSMError != "" && progress.SSM != inventory.FetchSkipped {
			fmt.Fprintf(w, "region %s SSM: %s\n", progress.Region, progress.SSMError)
		}
	}
}

func completedRegions(snapshot inventory.InventorySnapshot) int {
	n := 0
	for _, p := range snapshot.Progress {
		if p.EC2 == inventory.FetchSucceeded && (p.SSM == inventory.FetchSucceeded || p.SSM == inventory.FetchSkipped || p.SSM == inventory.FetchFailed || p.SSM == inventory.FetchCanceled) {
			n++
		}
	}
	return n
}
func nullable(v *string) string {
	if v == nil {
		return "null"
	}
	return strconv.Quote(*v)
}
