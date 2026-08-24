package configs

import (
	"strings"
	"testing"
)

// TestGetBootstrappers_EmbeddedMainnet proves the bootstrapper list travels in
// the binary via the embed — a node in a container (which bakes no
// bootstrappers.json on disk) still finds peers.
func TestGetBootstrappers_EmbeddedMainnet(t *testing.T) {
	got := GetBootstrappers(MainnetID)
	if len(got) == 0 {
		t.Fatal("mainnet bootstrappers must resolve from the embed, got none")
	}
	for _, b := range got {
		if !strings.HasPrefix(b.ID, "NodeID-") {
			t.Errorf("bootstrapper id must be a NodeID, got %q", b.ID)
		}
		if !strings.Contains(b.IP, ":") {
			t.Errorf("bootstrapper ip must be host:port, got %q", b.IP)
		}
	}
}

func TestGetBootstrappers_EmbeddedTestnet(t *testing.T) {
	if len(GetBootstrappers(TestnetID)) == 0 {
		t.Fatal("testnet bootstrappers must resolve from the embed, got none")
	}
}

// A network with no embedded list returns nil, so the caller falls back to disk
// paths or an explicit --bootstrap-nodes rather than crashing.
func TestGetBootstrappers_UnknownNetworkIsNil(t *testing.T) {
	if GetBootstrappers(0xF00D) != nil {
		t.Error("unknown network must return nil bootstrappers")
	}
}
