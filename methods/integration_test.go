package cloudops

import (
	"os"
	"testing"
)

// TestIntegration runs against a live SolidFire cluster if env vars are set.
// Env vars:
// SF_CONF: content of the yaml config or path to it, or simply use defaults if simple enough?
// Actually the NewClient takes a yaml string.
//
// Example usage:
// export SF_CONFIG="
// endpoint: https://admin:admin@192.168.1.34/json-rpc/12.5
// svip: 10.10.10.10:3260
// tenantname: test
// "
// go test -v ./methods/...

func TestGetClusterVersion(t *testing.T) {
	config := os.Getenv("SF_CONFIG")
	if config == "" {
		t.Skip("Skipping integration test: SF_CONFIG not set")
	}

	c, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	version, err := c.GetClusterVersion()
	if err != nil {
		t.Errorf("GetClusterVersion failed: %v", err)
	}
	t.Logf("Cluster Version: %s", version)
}

func TestListISCSISessions(t *testing.T) {
	config := os.Getenv("SF_CONFIG")
	if config == "" {
		t.Skip("Skipping integration test: SF_CONFIG not set")
	}

	c, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	sessions, err := c.ListISCSISessions()
	if err != nil {
		t.Errorf("ListISCSISessions failed: %v", err)
	}
	t.Logf("Found %d sessions", len(sessions))
	for _, s := range sessions {
		if s.Authentication != nil {
			t.Logf("Session %d Auth: %s / %s", s.SessionID, s.Authentication.AuthMethod, s.Authentication.ChapAlgorithm)
		}
	}
}
