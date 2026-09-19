package awsapi

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/uncho/ssmm/internal/target"
)

type fakeEC2Client struct {
	pages []*ec2.DescribeInstancesOutput
	index int
}

func (f *fakeEC2Client) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	page := f.pages[f.index]
	f.index++
	return page, nil
}

func TestEC2ReadsEmptyPagesUntilTokenEnds(t *testing.T) {
	client := &fakeEC2Client{pages: []*ec2.DescribeInstancesOutput{
		{NextToken: aws.String("next")},
		{Reservations: []types.Reservation{{Instances: []types.Instance{{InstanceId: aws.String("i-01234567"), State: &types.InstanceState{Name: types.InstanceStateNameRunning}}}}}},
	}}
	var pages int
	err := NewEC2(client).EC2(context.Background(), "us-east-1", target.TargetQuery{Kind: target.QueryAll}, func(rows []target.Instance) error {
		pages++
		if len(rows) == 1 && rows[0].Key.InstanceID != "i-01234567" {
			t.Fatal("unexpected instance")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("expected two callbacks, got %d", pages)
	}
}

func TestEC2RejectsRepeatedToken(t *testing.T) {
	client := &fakeEC2Client{pages: []*ec2.DescribeInstancesOutput{{NextToken: aws.String("same")}, {NextToken: aws.String("same")}}}
	err := NewEC2(client).EC2(context.Background(), "us-east-1", target.TargetQuery{Kind: target.QueryAll}, func([]target.Instance) error { return nil })
	if err == nil {
		t.Fatal("repeated pagination token should fail")
	}
}
