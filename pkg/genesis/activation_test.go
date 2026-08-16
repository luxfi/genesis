// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/json"
	"os"
	"testing"
)

// The precompile schedule and the staking policy share one activation. This
// reads the mainnet upgrade schedule and refuses any entry that names another.
func TestMainnetUpgradeScheduleActivatesAtTheOneInstant(t *testing.T) {
	raw, err := os.ReadFile("../../configs/mainnet/upgrade.json")
	if err != nil {
		t.Skipf("upgrade.json not beside the package: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("upgrade.json: %v", err)
	}
	var seen int
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if ts, ok := x["blockTimestamp"]; ok {
				seen++
				if f, ok := ts.(float64); !ok || int64(f) != Activation {
					t.Fatalf("a precompile activates at %v, not at Activation (%d)", ts, Activation)
				}
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(doc)
	if seen == 0 {
		t.Fatal("no blockTimestamp found in the mainnet upgrade schedule")
	}
}
