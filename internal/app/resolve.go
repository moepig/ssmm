package app

import (
	"fmt"

	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

func Candidates(snapshot inventory.InventorySnapshot, query target.TargetQuery, filter string) []target.Instance {
	rows, _ := target.Resolve(snapshot.Instances, query, filter)
	return rows
}

func ResolveSnapshot(profile target.ProfileSelection, snapshot inventory.InventorySnapshot, query target.TargetQuery, filter string, selected *target.InstanceKey, method target.ResolvedMethod) (target.ResolvedTarget, error) {
	candidates := Candidates(snapshot, query, filter)
	if selected != nil {
		for _, row := range candidates {
			if row.Key == *selected {
				if !row.Running() {
					return target.ResolvedTarget{}, fmt.Errorf("instance %s is %s and cannot be selected", row.Key, row.EC2State)
				}
				i := row
				return target.ResolvedTarget{Profile: profile, Region: row.Key.Region, InstanceID: row.Key.InstanceID, Method: target.ResolvedManual, Instance: &i}, nil
			}
		}
		return target.ResolvedTarget{}, fmt.Errorf("selected instance is not in the current candidates")
	}
	if !snapshot.Finished || !snapshot.EC2Complete {
		return target.ResolvedTarget{}, fmt.Errorf("search result is incomplete")
	}
	if len(candidates) == 0 {
		return target.ResolvedTarget{}, fmt.Errorf("no matching instance")
	}
	if len(candidates) != 1 {
		return target.ResolvedTarget{}, fmt.Errorf("target is ambiguous: %d instances match", len(candidates))
	}
	if !candidates[0].Running() {
		return target.ResolvedTarget{}, fmt.Errorf("instance %s is %s and cannot be selected", candidates[0].Key, candidates[0].EC2State)
	}
	i := candidates[0]
	return target.ResolvedTarget{Profile: profile, Region: i.Key.Region, InstanceID: i.Key.InstanceID, Method: method, Instance: &i}, nil
}
