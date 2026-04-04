package configs

import (
	"testing"
)

func TestListEmbed(t *testing.T) {
	entries, err := embeddedGenesis.ReadDir("mainnet")
	if err != nil {
		t.Fatalf("Error reading mainnet dir: %v", err)
	}
	t.Logf("Files embedded in mainnet/:")
	for _, e := range entries {
		info, _ := e.Info()
		t.Logf("  - %s (%d bytes)", e.Name(), info.Size())
	}
}
