package inventory

import (
	"context"
	"fmt"
	"sync"

	"github.com/uncho/ssmm/internal/target"
)

type eventKind uint8

const (
	ec2Page eventKind = iota
	ec2Done
	ssmPage
	ssmDone
)

type event struct {
	kind   eventKind
	region string
	page   []target.Instance
	ssm    []SSMRecord
	state  FetchState
	err    error
}

type Run struct {
	Snapshots <-chan InventorySnapshot
	Done      <-chan struct{}
	cancel    context.CancelFunc
}

func (r *Run) Cancel() {
	if r != nil && r.cancel != nil {
		r.cancel()
	}
}

// Start begins one generation. Only the aggregator mutates the inventory
// state; source callbacks send complete pages as events.
func Start(parent context.Context, generation uint64, scope SearchScope, q target.TargetQuery, source Source) (*Run, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("inventory source is nil")
	}
	ctx, cancel := context.WithCancel(parent)
	events := make(chan event, 32)
	snapshots := make(chan InventorySnapshot, 1)
	done := make(chan struct{})

	go func() {
		defer close(events)
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		for _, region := range scope.Regions {
			region := region
			wg.Add(1)
			go func() {
				defer wg.Done()
				found := 0
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() { <-sem }()
				send := func(e event) error {
					select {
					case events <- e:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				ec2Err := source.EC2(ctx, region, q, func(page []target.Instance) error {
					found += len(page)
					cloned := make([]target.Instance, len(page))
					for i := range page {
						cloned[i] = page[i].Clone()
					}
					return send(event{kind: ec2Page, region: region, page: cloned})
				})
				if ec2Err != nil {
					_ = send(event{kind: ec2Done, region: region, state: stateForError(ctx, ec2Err), err: ec2Err})
					_ = send(event{kind: ssmDone, region: region, state: FetchSkipped, err: fmt.Errorf("SSM skipped because EC2 failed: %w", ec2Err)})
					return
				}
				if err := send(event{kind: ec2Done, region: region, state: FetchSucceeded}); err != nil {
					return
				}
				if found == 0 {
					_ = send(event{kind: ssmDone, region: region, state: FetchSkipped})
					return
				}
				ssmErr := source.SSM(ctx, region, func(page []SSMRecord) error {
					return send(event{kind: ssmPage, region: region, ssm: append([]SSMRecord(nil), page...)})
				})
				if ssmErr != nil {
					_ = send(event{kind: ssmDone, region: region, state: stateForError(ctx, ssmErr), err: ssmErr})
					return
				}
				_ = send(event{kind: ssmDone, region: region, state: FetchSucceeded})
			}()
		}
		wg.Wait()
	}()

	go aggregate(ctx, cancel, generation, scope, events, snapshots, done)
	return &Run{Snapshots: snapshots, Done: done, cancel: cancel}, nil
}

func stateForError(ctx context.Context, err error) FetchState {
	if ctx.Err() != nil || err == context.Canceled || err == context.DeadlineExceeded {
		return FetchCanceled
	}
	return FetchFailed
}

func Collect(ctx context.Context, generation uint64, scope SearchScope, q target.TargetQuery, source Source) (InventorySnapshot, error) {
	run, err := Start(ctx, generation, scope, q, source)
	if err != nil {
		return InventorySnapshot{}, err
	}
	var last InventorySnapshot
	for snapshot := range run.Snapshots {
		last = snapshot
	}
	<-run.Done
	if last.Revision == 0 {
		return last, fmt.Errorf("inventory ended without a snapshot")
	}
	return last, nil
}
