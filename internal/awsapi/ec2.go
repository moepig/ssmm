package awsapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/uncho/ssmm/internal/target"
)

type EC2API interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type EC2 struct {
	client      EC2API
	PageTimeout time.Duration
}

func NewEC2(client EC2API) EC2 { return EC2{client: client, PageTimeout: 30 * time.Second} }

func (e EC2) EC2(ctx context.Context, region string, query target.TargetQuery, callback func([]target.Instance) error) error {
	if e.client == nil {
		return fmt.Errorf("EC2 client is nil")
	}
	if callback == nil {
		return fmt.Errorf("EC2 callback is nil")
	}
	filters := []types.Filter{{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "shutting-down", "stopping", "stopped"}}}
	switch query.Kind {
	case target.QueryID:
		filters = append(filters, types.Filter{Name: aws.String("instance-id"), Values: []string{query.Value}})
	case target.QueryName:
		if !strings.ContainsAny(query.Value, "*?") {
			filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{query.Value}})
		}
	}
	for _, tag := range query.Tags {
		if strings.ContainsAny(tag.Key, "*?") || strings.ContainsAny(tag.Value, "*?") {
			continue
		}
		filters = append(filters, types.Filter{Name: aws.String("tag:" + tag.Key), Values: []string{tag.Value}})
	}
	for i := 0; i < len(query.Tags); i++ {
		for j := i + 1; j < len(query.Tags); j++ {
			if query.Tags[i].Key == query.Tags[j].Key && query.Tags[i].Value != query.Tags[j].Value {
				return nil
			}
		}
	}

	var token *string
	seenTokens := map[string]bool{}
	for {
		if token != nil && seenTokens[*token] {
			return fmt.Errorf("%w: EC2 pagination token repeated in %s", ErrPaginationTokenRepeated, region)
		}
		if token != nil {
			seenTokens[*token] = true
		}
		pageCtx := ctx
		cancel := func() {}
		if e.PageTimeout > 0 {
			pageCtx, cancel = context.WithTimeout(ctx, e.PageTimeout)
		}
		output, err := e.client.DescribeInstances(pageCtx, &ec2.DescribeInstancesInput{Filters: filters, NextToken: token})
		cancel()
		if err != nil {
			return fmt.Errorf("describe instances in %s: %w", region, err)
		}
		rows := make([]target.Instance, 0)
		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				if row, ok := convertInstance(region, instance); ok && target.MatchesQuery(row, query) {
					rows = append(rows, row)
				}
			}
		}
		if err := callback(rows); err != nil {
			return err
		}
		next := aws.ToString(output.NextToken)
		if next == "" {
			return nil
		}
		token = aws.String(next)
	}
}

func convertInstance(region string, instance types.Instance) (target.Instance, bool) {
	if instance.InstanceId == nil || instance.State == nil {
		return target.Instance{}, false
	}
	state := string(instance.State.Name)
	if state == "terminated" {
		return target.Instance{}, false
	}
	row := target.Instance{Key: target.InstanceKey{Region: region, InstanceID: aws.ToString(instance.InstanceId)}, EC2State: state, SSMStatus: target.SSMUnknown, PrivateIP: instance.PrivateIpAddress}
	if instance.Placement != nil {
		row.AvailabilityZone = instance.Placement.AvailabilityZone
	}
	row.Tags = make(map[string]string, len(instance.Tags))
	for _, tag := range instance.Tags {
		if tag.Key == nil {
			continue
		}
		row.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	if name, ok := row.Tags["Name"]; ok {
		row.Name = aws.String(name)
	}
	return row, true
}
