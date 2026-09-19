package awsapi

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type RegionsAPI interface {
	DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

type Regions struct {
	client      RegionsAPI
	PageTimeout time.Duration
}

func NewRegions(client RegionsAPI) Regions {
	return Regions{client: client, PageTimeout: 30 * time.Second}
}

func (r Regions) EnabledRegions(ctx context.Context) ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("EC2 client is nil")
	}
	requestCtx := ctx
	cancel := func() {}
	if r.PageTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, r.PageTimeout)
	}
	out, err := r.client.DescribeRegions(requestCtx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	cancel()
	if err != nil {
		return nil, fmt.Errorf("describe regions: %w", err)
	}
	regions := make([]string, 0, len(out.Regions))
	for _, region := range out.Regions {
		if region.RegionName == nil || *region.RegionName == "" {
			continue
		}
		optInStatus := aws.ToString(region.OptInStatus)
		if optInStatus != "" && optInStatus != "opt-in-not-required" && optInStatus != "opted-in" {
			continue
		}
		regions = append(regions, *region.RegionName)
	}
	sort.Strings(regions)
	return regions, nil
}

func regionFilter(name, value string) types.Filter {
	return types.Filter{Name: aws.String(name), Values: []string{value}}
}
