package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/uncho/ssmm/internal/target"
)

type fakeSource struct {
	ec2      map[string][][]target.Instance
	ssm      map[string][][]SSMRecord
	ec2Err   map[string]error
	ssmErr   map[string]error
	ssmCalls int
}

func (f *fakeSource) EC2(_ context.Context, region string, _ target.TargetQuery, callback func([]target.Instance) error) error {
	for _, page := range f.ec2[region] {
		if err := callback(page); err != nil {
			return err
		}
	}
	return f.ec2Err[region]
}
func (f *fakeSource) SSM(_ context.Context, region string, callback func([]SSMRecord) error) error {
	f.ssmCalls++
	for _, page := range f.ssm[region] {
		if err := callback(page); err != nil {
			return err
		}
	}
	return f.ssmErr[region]
}

func TestCollectSeparatesEC2FailureAndSSMStatus(t *testing.T) {
	name := "web"
	source := &fakeSource{ec2: map[string][][]target.Instance{"good": {{{Key: target.InstanceKey{Region: "good", InstanceID: "i-01234567"}, Name: &name, EC2State: "running"}}}, "bad": {{{Key: target.InstanceKey{Region: "bad", InstanceID: "i-01234568"}, EC2State: "running"}}}}, ssm: map[string][][]SSMRecord{"good": {{{InstanceID: "i-01234567", Status: target.SSMOnline}}}, "bad": nil}, ec2Err: map[string]error{"bad": errors.New("unavailable")}, ssmErr: map[string]error{}}
	snapshot, err := Collect(context.Background(), 4, SearchScope{DiscoveryRegion: "good", Regions: []string{"good", "bad"}}, target.TargetQuery{Kind: target.QueryAll}, source)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.EC2Complete || !snapshot.Finished {
		t.Fatalf("unexpected completion state: %#v", snapshot)
	}
	if source.ssmCalls != 1 {
		t.Fatalf("SSM should be skipped for failed or empty EC2 regions, calls=%d", source.ssmCalls)
	}
	if len(snapshot.Instances) != 2 {
		t.Fatalf("expected rows from both EC2 pages, got %d", len(snapshot.Instances))
	}
}

type blockingSource struct{}

func (blockingSource) EC2(ctx context.Context, _ string, _ target.TargetQuery, _ func([]target.Instance) error) error {
	<-ctx.Done()
	return ctx.Err()
}
func (blockingSource) SSM(ctx context.Context, _ string, _ func([]SSMRecord) error) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestStartPublishesRunningSnapshotBeforeFetchCompletes(t *testing.T) {
	run, err := Start(context.Background(), 9, SearchScope{DiscoveryRegion: "us-east-1", Regions: []string{"us-east-1"}}, target.TargetQuery{Kind: target.QueryAll}, blockingSource{})
	if err != nil {
		t.Fatal(err)
	}
	first := <-run.Snapshots
	if first.Generation != 9 || first.Finished || first.Revision == 0 {
		t.Fatalf("unexpected initial snapshot: %#v", first)
	}
	run.Cancel()
	select {
	case <-run.Done:
	case <-time.After(time.Second):
		t.Fatal("canceled inventory did not finish")
	}
}
