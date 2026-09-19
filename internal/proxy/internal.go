package proxy

import (
	"fmt"
	"strconv"
)

func ParseInternal(args []string, environProfile string) (InternalRequest, error) {
	var req InternalRequest
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		if key != "--profile" && key != "--internal-profile-source" && key != "--region" && key != "--port" {
			if req.InstanceID == "" {
				req.InstanceID = key
				continue
			}
			return InternalRequest{}, fmt.Errorf("unexpected internal proxy argument %q", key)
		}
		if i+1 >= len(args) {
			return InternalRequest{}, fmt.Errorf("missing value for %s", key)
		}
		value := args[i+1]
		i++
		switch key {
		case "--profile":
			req.Profile = value
		case "--internal-profile-source":
			req.Source = value
		case "--region":
			req.Region = value
		case "--port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return InternalRequest{}, fmt.Errorf("invalid port: %w", err)
			}
			req.Port = port
		}
		seen[key] = true
	}
	if req.InstanceID == "" || !seen["--profile"] || !seen["--internal-profile-source"] || !seen["--region"] || !seen["--port"] {
		return InternalRequest{}, fmt.Errorf("internal proxy requires ID, profile, source, region, and port")
	}
	if err := req.Validate(environProfile); err != nil {
		return InternalRequest{}, err
	}
	return req, nil
}
