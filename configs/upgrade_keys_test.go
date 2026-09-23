package configs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// upgradeFile holds the top-level keys of an upgrade.json and nothing else:
// the same three fields as luxfi/evm's params/extras UpgradeConfig. The C-Chain
// VM decodes upgrade bytes with DisallowUnknownFields (plugin/evm/vm.go), so a
// key it does not know stops the chain from initializing at all.
type upgradeFile struct {
	NetworkUpgradeOverrides json.RawMessage `json:"networkUpgradeOverrides,omitempty"`
	StateUpgrades           json.RawMessage `json:"stateUpgrades,omitempty"`
	PrecompileUpgrades      json.RawMessage `json:"precompileUpgrades,omitempty"`
}

// Every upgrade.json in the tree, embedded or not, decodes the way the VM
// decodes it. local/ is not embedded but is handed to nodes by path.
func TestUpgradeFilesDecodeStrictly(t *testing.T) {
	paths, err := filepath.Glob("*/upgrade.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no */upgrade.json found")
	}
	for _, path := range paths {
		t.Run(filepath.Dir(path), func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			dec := json.NewDecoder(bytes.NewReader(b))
			dec.DisallowUnknownFields()
			var u upgradeFile
			if err := dec.Decode(&u); err != nil {
				t.Fatalf("%s: the VM would refuse this file: %v", path, err)
			}
		})
	}
}
