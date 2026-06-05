// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoBrandShadow guards Task #109: brand L1 genesis lives in per-brand
// universe repos (hanzoai/universe, zooai/universe), not in luxfi/genesis.
//
// It walks ./configs/ at repo root and asserts no genesis.json sits under
// any top-level configs/hanzo-* or configs/zoo-* directory. Tombstone
// MOVED.txt files are allowed (and expected); only re-introduced
// genesis.json shadows fail the test.
//
// pars-*/spc-* are explicitly allowed under configs/_orphan-l2/ since
// no parsdao/universe or spc-universe repo exists yet (see
// configs/_orphan-l2/README.md).
func TestNoBrandShadow(t *testing.T) {
	configsDir := configsDirFromCaller(t)

	entries, err := os.ReadDir(configsDir)
	if err != nil {
		t.Fatalf("read configs dir %q: %v", configsDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !brandTopLevelName(name) {
			continue
		}
		dir := filepath.Join(configsDir, name)
		if hasGenesisJSON(t, dir) {
			t.Errorf("brand shadow detected: %s/genesis.json exists at top level configs/; "+
				"per Task #109, brand L1 genesis must live in hanzoai/universe or zooai/universe, "+
				"not in luxfi/genesis. If this is a pars-*/spc-* dir, move it under configs/_orphan-l2/.",
				name)
		}
	}
}

// brandTopLevelName matches the top-level brand prefixes we forbid:
// hanzo-* and zoo-*. pars-* and spc-* are also disallowed at top level
// (they belong under configs/_orphan-l2/) but the design only flagged
// hanzo/zoo as having a per-brand repo today; we include pars/spc too
// since the orphan move happened in the same commit.
func brandTopLevelName(name string) bool {
	for _, prefix := range []string{"hanzo-", "zoo-", "pars-", "spc-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasGenesisJSON(t *testing.T, dir string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, "genesis.json"))
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s/genesis.json: %v", dir, err)
	}
	return false
}

// configsDirFromCaller locates ../../configs relative to the test file,
// which puts it at the genesis repo root regardless of where `go test`
// is invoked from.
func configsDirFromCaller(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = .../lux/genesis/pkg/genesis/no_brand_shadow_test.go
	// configs   = .../lux/genesis/configs
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "configs"))
}
