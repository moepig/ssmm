package target

import (
	"fmt"
	"sort"
)

type InstanceKey struct {
	Region     string `json:"region"`
	InstanceID string `json:"instance_id"`
}

func (k InstanceKey) String() string { return k.Region + "/" + k.InstanceID }

func (k InstanceKey) Valid() bool { return k.Region != "" && k.InstanceID != "" }

type SSMStatus string

const (
	SSMUnknown     SSMStatus = "Unknown"
	SSMOnline      SSMStatus = "Online"
	SSMConnection  SSMStatus = "ConnectionLost"
	SSMNotReported SSMStatus = "NotReported"
)

type Instance struct {
	Key              InstanceKey
	Name             *string
	EC2State         string
	SSMStatus        SSMStatus
	PrivateIP        *string
	AvailabilityZone *string
	Tags             map[string]string
}

func (i Instance) Running() bool { return i.EC2State == "running" }

func (i Instance) Clone() Instance {
	c := i
	if i.Name != nil {
		v := *i.Name
		c.Name = &v
	}
	if i.PrivateIP != nil {
		v := *i.PrivateIP
		c.PrivateIP = &v
	}
	if i.AvailabilityZone != nil {
		v := *i.AvailabilityZone
		c.AvailabilityZone = &v
	}
	c.Tags = make(map[string]string, len(i.Tags))
	for k, v := range i.Tags {
		c.Tags[k] = v
	}
	return c
}

type ResolvedMethod string

const (
	ResolvedUnique ResolvedMethod = "unique"
	ResolvedManual ResolvedMethod = "manual"
	ResolvedDirect ResolvedMethod = "direct"
)

type ResolvedTarget struct {
	Profile    ProfileSelection
	Region     string
	InstanceID string
	Method     ResolvedMethod
	Instance   *Instance
}

func (t ResolvedTarget) Validate() error {
	if err := t.Profile.Validate(); err != nil {
		return err
	}
	if t.Region == "" || t.InstanceID == "" {
		return fmt.Errorf("resolved target requires region and instance ID")
	}
	if !IsInstanceID(t.InstanceID) {
		return fmt.Errorf("invalid instance ID %q", t.InstanceID)
	}
	if t.Method != ResolvedUnique && t.Method != ResolvedManual && t.Method != ResolvedDirect {
		return fmt.Errorf("invalid target resolution method %q", t.Method)
	}
	if t.Instance != nil && t.Instance.Key != (InstanceKey{Region: t.Region, InstanceID: t.InstanceID}) {
		return fmt.Errorf("resolved target instance does not match target key")
	}
	return nil
}

func Sort(instances []Instance) []Instance {
	out := make([]Instance, len(instances))
	for i := range instances {
		out[i] = instances[i].Clone()
	}
	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := stringValue(out[i].Name), stringValue(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		if out[i].Key.Region != out[j].Key.Region {
			return out[i].Key.Region < out[j].Key.Region
		}
		return out[i].Key.InstanceID < out[j].Key.InstanceID
	})
	return out
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
