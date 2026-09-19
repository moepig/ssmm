package inventory

import (
	"context"
	"fmt"

	"github.com/uncho/ssmm/internal/target"
)

type SSMRecord struct {
	InstanceID string
	Status     target.SSMStatus
}

type Source interface {
	EC2(context.Context, string, target.TargetQuery, func([]target.Instance) error) error
	SSM(context.Context, string, func([]SSMRecord) error) error
}

type SearchScope struct {
	DiscoveryRegion string
	Regions         []string
	Source          string
}

func (s SearchScope) Validate() error {
	if s.DiscoveryRegion == "" || len(s.Regions) == 0 {
		return fmt.Errorf("search scope must contain a discovery region and at least one region")
	}
	seen := make(map[string]struct{}, len(s.Regions))
	for _, r := range s.Regions {
		if r == "" {
			return fmt.Errorf("search scope contains an empty region")
		}
		if _, ok := seen[r]; ok {
			return fmt.Errorf("search scope contains duplicate region %q", r)
		}
		seen[r] = struct{}{}
	}
	return nil
}
