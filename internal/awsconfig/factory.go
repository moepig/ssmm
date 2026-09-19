package awsconfig

import (
	"context"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/uncho/ssmm/internal/awsapi"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

type Clients struct {
	EC2 *ec2.Client
	SSM *ssm.Client
}

type Runtime struct {
	cfg       awssdk.Config
	selection target.ProfileSelection
	endpoint  string
	mu        sync.Mutex
	clients   map[string]Clients
}

func (r *Runtime) Selection() target.ProfileSelection { return r.selection }

func (r *Runtime) clientsFor(region string) Clients {
	r.mu.Lock()
	defer r.mu.Unlock()
	if clients, ok := r.clients[region]; ok {
		return clients
	}
	cfg := r.cfg.Copy()
	cfg.Region = region
	ec2Options := make([]func(*ec2.Options), 0, 1)
	ssmOptions := make([]func(*ssm.Options), 0, 1)
	if r.endpoint != "" {
		ec2Options = append(ec2Options, func(options *ec2.Options) {
			options.BaseEndpoint = awssdk.String(r.endpoint)
		})
		ssmOptions = append(ssmOptions, func(options *ssm.Options) {
			options.BaseEndpoint = awssdk.String(r.endpoint)
		})
	}
	clients := Clients{EC2: ec2.NewFromConfig(cfg, ec2Options...), SSM: ssm.NewFromConfig(cfg, ssmOptions...)}
	r.clients[region] = clients
	return clients
}

func (r *Runtime) EC2(ctx context.Context, region string, query target.TargetQuery, page func([]target.Instance) error) error {
	clients := r.clientsFor(region)
	return awsapi.NewEC2(clients.EC2).EC2(ctx, region, query, page)
}

func (r *Runtime) SSM(ctx context.Context, region string, page func([]inventory.SSMRecord) error) error {
	clients := r.clientsFor(region)
	return awsapi.NewSSM(clients.SSM).SSM(ctx, region, page)
}

func (r *Runtime) EnabledRegions(ctx context.Context, discoveryRegion string) ([]string, error) {
	region := discoveryRegion
	if region == "" {
		region = DiscoveryRegion(r.cfg)
	}
	cfg := r.cfg.Copy()
	cfg.Region = region
	options := make([]func(*ec2.Options), 0, 1)
	if r.endpoint != "" {
		options = append(options, func(clientOptions *ec2.Options) {
			clientOptions.BaseEndpoint = awssdk.String(r.endpoint)
		})
	}
	return awsapi.NewRegions(ec2.NewFromConfig(cfg, options...)).EnabledRegions(ctx)
}

var _ inventory.Source = (*Runtime)(nil)
