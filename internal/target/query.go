package target

import "fmt"

type QueryKind string

const (
	QueryAll  QueryKind = "all"
	QueryName QueryKind = "name"
	QueryID   QueryKind = "id"
)

type TagFilter struct {
	Key   string
	Value string
}

type TargetQuery struct {
	Kind  QueryKind
	Value string
	Tags  []TagFilter
}

func ParseQuery(value string, tags []TagFilter) (TargetQuery, error) {
	if value == "" {
		if len(tags) == 0 {
			return TargetQuery{Kind: QueryAll}, nil
		}
		return TargetQuery{Kind: QueryAll, Tags: cloneTags(tags)}, nil
	}
	kind := QueryName
	if IsInstanceID(value) {
		kind = QueryID
		if len(tags) != 0 {
			return TargetQuery{}, fmt.Errorf("ID cannot be combined with tag filters")
		}
	}
	for _, tag := range tags {
		if tag.Key == "" || tag.Value == "" {
			return TargetQuery{}, fmt.Errorf("tag must be KEY=VALUE with non-empty key and value")
		}
	}
	return TargetQuery{Kind: kind, Value: value, Tags: cloneTags(tags)}, nil
}

func IsInstanceID(s string) bool {
	if len(s) != 10 && len(s) != 19 {
		return false
	}
	if len(s) < 2 || s[:2] != "i-" {
		return false
	}
	for _, r := range s[2:] {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func cloneTags(tags []TagFilter) []TagFilter {
	return append([]TagFilter(nil), tags...)
}
