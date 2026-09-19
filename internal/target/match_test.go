package target

import "testing"

func TestMatchesQueryAndPartialFilter(t *testing.T) {
	name := "web-api"
	ip := "10.0.0.1"
	row := Instance{Key: InstanceKey{Region: "ap-northeast-1", InstanceID: "i-01234567"}, Name: &name, EC2State: "running", PrivateIP: &ip, Tags: map[string]string{"Environment": "prod"}}
	query, err := ParseQuery("web-api", []TagFilter{{Key: "Environment", Value: "prod"}})
	if err != nil || !MatchesQuery(row, query) {
		t.Fatalf("expected exact query to match: %v", err)
	}
	if MatchesQuery(row, TargetQuery{Kind: QueryName, Value: "WEB-API"}) {
		t.Fatal("name matching must be case-sensitive")
	}
	if !PartialMatch(row, "WEB prod") {
		t.Fatal("partial filter should be case-insensitive across tags")
	}
}

func TestIDDetection(t *testing.T) {
	if !IsInstanceID("i-01234567") || !IsInstanceID("i-0123456789abcdef0") {
		t.Fatal("valid EC2 IDs were rejected")
	}
	if IsInstanceID("i-0123456g") || IsInstanceID("web") {
		t.Fatal("invalid EC2 ID was accepted")
	}
}
