package target

import "strings"

// MatchesQuery applies the exact AWS-side query semantics locally as well.
// Name and tag comparisons are case-sensitive; wildcard-looking Names are
// intentionally treated as literal values.
func MatchesQuery(i Instance, q TargetQuery) bool {
	switch q.Kind {
	case QueryID:
		if i.Key.InstanceID != q.Value {
			return false
		}
	case QueryName:
		if i.Name == nil || *i.Name != q.Value {
			return false
		}
	case QueryAll:
	default:
		return false
	}
	for _, tag := range q.Tags {
		if i.Tags == nil || i.Tags[tag.Key] != tag.Value {
			return false
		}
	}
	return true
}

func PartialMatch(i Instance, filter string) bool {
	words := strings.Fields(strings.ToLower(filter))
	if len(words) == 0 {
		return true
	}
	fields := []string{stringValue(i.Name), i.Key.Region, i.Key.InstanceID, stringValue(i.PrivateIP), stringValue(i.AvailabilityZone)}
	for k, v := range i.Tags {
		fields = append(fields, k, v)
	}
	for _, word := range words {
		found := false
		for _, field := range fields {
			if strings.Contains(strings.ToLower(field), word) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func Filter(instances []Instance, filter string) []Instance {
	out := make([]Instance, 0, len(instances))
	for _, i := range instances {
		if PartialMatch(i, filter) {
			out = append(out, i.Clone())
		}
	}
	return out
}

func Resolve(instances []Instance, q TargetQuery, filter string) ([]Instance, error) {
	matched := make([]Instance, 0)
	for _, i := range instances {
		if MatchesQuery(i, q) && PartialMatch(i, filter) {
			matched = append(matched, i.Clone())
		}
	}
	return Sort(matched), nil
}
