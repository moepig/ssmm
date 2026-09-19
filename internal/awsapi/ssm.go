package awsapi

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type SSMAPI interface {
	DescribeInstanceInformation(context.Context, *ssm.DescribeInstanceInformationInput, ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

type SSM struct {
	client      SSMAPI
	PageTimeout time.Duration
}

func NewSSM(client SSMAPI) SSM { return SSM{client: client, PageTimeout: 30 * time.Second} }

func (s SSM) SSM(ctx context.Context, region string, callback func([]inventory.SSMRecord) error) error {
	if s.client == nil {
		return fmt.Errorf("SSM client is nil")
	}
	if callback == nil {
		return fmt.Errorf("SSM callback is nil")
	}
	var token *string
	seenTokens := map[string]bool{}
	for {
		if token != nil && seenTokens[*token] {
			return fmt.Errorf("%w: SSM pagination token repeated in %s", ErrPaginationTokenRepeated, region)
		}
		if token != nil {
			seenTokens[*token] = true
		}
		pageCtx := ctx
		cancel := func() {}
		if s.PageTimeout > 0 {
			pageCtx, cancel = context.WithTimeout(ctx, s.PageTimeout)
		}
		output, err := s.client.DescribeInstanceInformation(pageCtx, &ssm.DescribeInstanceInformationInput{NextToken: token})
		cancel()
		if err != nil {
			return fmt.Errorf("describe SSM information in %s: %w", region, err)
		}
		rows := make([]inventory.SSMRecord, 0, len(output.InstanceInformationList))
		for _, info := range output.InstanceInformationList {
			if info.InstanceId == nil {
				continue
			}
			status := target.SSMUnknown
			if info.PingStatus == ssmtypes.PingStatusOnline {
				status = target.SSMOnline
			} else if info.PingStatus == ssmtypes.PingStatusConnectionLost || info.PingStatus == ssmtypes.PingStatusInactive {
				status = target.SSMConnection
			}
			rows = append(rows, inventory.SSMRecord{InstanceID: aws.ToString(info.InstanceId), Status: status})
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
