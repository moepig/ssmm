package inventory

import (
	"sort"

	"github.com/uncho/ssmm/internal/target"
)

type FetchState string

const (
	FetchNotStarted FetchState = "not-started"
	FetchRunning    FetchState = "running"
	FetchSucceeded  FetchState = "succeeded"
	FetchFailed     FetchState = "failed"
	FetchSkipped    FetchState = "skipped"
	FetchCanceled   FetchState = "canceled"
)

type RegionProgress struct {
	Region   string
	EC2      FetchState
	SSM      FetchState
	EC2Count int
	SSMCount int
	EC2Error string
	SSMError string
}

type InventorySnapshot struct {
	Generation  uint64
	Revision    uint64
	Instances   []target.Instance
	Progress    []RegionProgress
	EC2Complete bool
	Finished    bool
	Canceled    bool
}

func (s InventorySnapshot) Clone() InventorySnapshot {
	c := s
	c.Instances = make([]target.Instance, len(s.Instances))
	for i := range s.Instances {
		c.Instances[i] = s.Instances[i].Clone()
	}
	c.Progress = append([]RegionProgress(nil), s.Progress...)
	return c
}

func (s InventorySnapshot) progressMap() map[string]RegionProgress {
	m := make(map[string]RegionProgress, len(s.Progress))
	for _, p := range s.Progress {
		m[p.Region] = p
	}
	return m
}

func sortProgress(in []RegionProgress) []RegionProgress {
	out := append([]RegionProgress(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Region < out[j].Region })
	return out
}
