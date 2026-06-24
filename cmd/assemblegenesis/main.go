// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

// Command assemblegenesis writes the canonical combined genesis.json for a
// network by assembling it from the embedded split shards (network.json +
// pchain.json + {x,c,d,q,a,b,t,z,g,k}chain.json) via the exact same
// configs.GetGenesis path luxd uses at boot. The combined genesis.json is a
// human-facing snapshot; the shards are the source of truth. Run this after
// editing any shard so the snapshot stays faithful to what the node builds.
//
// Usage:
//
//	assemblegenesis -network-id 1 -output configs/mainnet/genesis.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/luxfi/genesis/configs"
)

func main() {
	networkID := flag.Uint("network-id", 0, "Network ID (1=mainnet, 2=testnet, 3=devnet, 1337=local)")
	output := flag.String("output", "", "Output file path (default: stdout)")
	flag.Parse()

	if *networkID == 0 {
		fmt.Fprintln(os.Stderr, "Error: -network-id is required")
		os.Exit(2)
	}

	// GetGenesis assembles from the embedded shards exactly as luxd does.
	data, err := configs.GetGenesis(uint32(*networkID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error assembling genesis: %v\n", err)
		os.Exit(1)
	}

	// Pretty-print for the human-facing snapshot (2-space indent, matching
	// the existing committed shape). Parsing is format-agnostic, so indent
	// choice is cosmetic — but consistency keeps diffs readable.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "Error pretty-printing genesis: %v\n", err)
		os.Exit(1)
	}
	pretty.WriteByte('\n')

	if *output == "" {
		os.Stdout.Write(pretty.Bytes())
		return
	}
	if err := os.WriteFile(*output, pretty.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", *output, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Wrote combined genesis for networkID %d to %s (%d bytes)\n",
		*networkID, *output, pretty.Len())
}
