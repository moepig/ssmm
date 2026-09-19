package inventory

import (
	"context"

	"github.com/uncho/ssmm/internal/target"
)

type aggregateState struct {
	instances map[target.InstanceKey]target.Instance
	ssm       map[string]map[string]target.SSMStatus
	progress  map[string]RegionProgress
}

func aggregate(ctx context.Context, cancel context.CancelFunc, generation uint64, scope SearchScope, events <-chan event, out chan InventorySnapshot, done chan<- struct{}) {
	defer close(done)
	defer close(out)
	defer cancel()
	state := aggregateState{
		instances: make(map[target.InstanceKey]target.Instance),
		ssm:       make(map[string]map[string]target.SSMStatus),
		progress:  make(map[string]RegionProgress, len(scope.Regions)),
	}
	for _, region := range scope.Regions {
		state.progress[region] = RegionProgress{Region: region, EC2: FetchRunning, SSM: FetchNotStarted}
	}
	var revision uint64
	emit := func(canceled bool, finished bool) {
		revision++
		rows := make([]target.Instance, 0, len(state.instances))
		for _, row := range state.instances {
			rows = append(rows, row.Clone())
		}
		rows = target.Sort(rows)
		progress := make([]RegionProgress, 0, len(state.progress))
		for _, p := range state.progress {
			progress = append(progress, p)
		}
		progress = sortProgress(progress)
		ec2Complete := true
		for _, p := range progress {
			if p.EC2 != FetchSucceeded {
				ec2Complete = false
				break
			}
		}
		s := InventorySnapshot{Generation: generation, Revision: revision, Instances: rows, Progress: progress, EC2Complete: ec2Complete, Finished: finished, Canceled: canceled}
		select {
		case out <- s:
		case old := <-out:
			_ = old
			out <- s
		case <-ctx.Done():
			// The final state is still emitted when possible. The receiver may
			// have stopped after a selected row, so never block shutdown.
			select {
			case out <- s:
			default:
			}
		}
	}
	// Publish an empty running snapshot immediately so interactive consumers
	// can open while the first region is still being fetched.
	emit(false, false)

	for e := range events {
		p := state.progress[e.region]
		switch e.kind {
		case ec2Page:
			for _, row := range e.page {
				if row.Key.Region == "" {
					row.Key.Region = e.region
				}
				state.instances[row.Key] = row.Clone()
			}
			p.EC2Count = countRegion(state.instances, e.region)
			p.EC2 = FetchRunning
		case ec2Done:
			p.EC2 = e.state
			if e.err != nil {
				p.EC2Error = e.err.Error()
			}
			if e.state == FetchSucceeded && p.EC2Count == 0 {
				p.SSM = FetchSkipped
			}
		case ssmPage:
			if state.ssm[e.region] == nil {
				state.ssm[e.region] = make(map[string]target.SSMStatus)
			}
			for _, record := range e.ssm {
				state.ssm[e.region][record.InstanceID] = record.Status
			}
			p.SSM = FetchRunning
			p.SSMCount = len(state.ssm[e.region])
		case ssmDone:
			p.SSM = e.state
			if e.err != nil {
				p.SSMError = e.err.Error()
			}
			if e.state == FetchSucceeded {
				for key, row := range state.instances {
					if key.Region != e.region {
						continue
					}
					if status, ok := state.ssm[e.region][key.InstanceID]; ok {
						row.SSMStatus = status
					} else {
						row.SSMStatus = target.SSMNotReported
					}
					state.instances[key] = row
				}
			} else {
				for key, row := range state.instances {
					if key.Region == e.region {
						row.SSMStatus = target.SSMUnknown
						state.instances[key] = row
					}
				}
			}
		}
		state.progress[e.region] = p
		finished := allFinished(state.progress, scope.Regions)
		emit(ctx.Err() != nil, finished)
	}

	canceled := ctx.Err() != nil
	for _, region := range scope.Regions {
		p := state.progress[region]
		if canceled {
			if p.EC2 == FetchRunning || p.EC2 == FetchNotStarted {
				p.EC2 = FetchCanceled
			}
			if p.SSM == FetchRunning || p.SSM == FetchNotStarted {
				p.SSM = FetchCanceled
			}
		}
		state.progress[region] = p
	}
	emit(canceled, true)
}

func countRegion(rows map[target.InstanceKey]target.Instance, region string) int {
	n := 0
	for key := range rows {
		if key.Region == region {
			n++
		}
	}
	return n
}

func allFinished(progress map[string]RegionProgress, regions []string) bool {
	for _, region := range regions {
		p := progress[region]
		if !terminal(p.EC2) || !terminal(p.SSM) {
			return false
		}
	}
	return true
}

func terminal(s FetchState) bool {
	return s == FetchSucceeded || s == FetchFailed || s == FetchSkipped || s == FetchCanceled
}
