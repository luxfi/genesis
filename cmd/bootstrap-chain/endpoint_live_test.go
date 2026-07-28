// Live proof that bootstrap-chain's read-only probes speak the node's one
// route prefix. luxd pins baseURL="/v1" (lux/node/server/http/server.go); it
// has never served "/ext". Every probe below therefore has to hit /v1 or the
// tool is blind against any current fleet — which it was, silently, because
// each probe reports a bad path as "not bootstrapped yet" rather than as an
// error.
//
// These run against the public mainnet gateway, read-only: getBlockchains,
// isBootstrapped and eth_blockNumber issue no transaction and cost nothing.
// `go test -short` skips them.

package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const liveURI = "https://api.lux.network"

func liveCtx(t *testing.T) (context.Context, *http.Client) {
	t.Helper()
	if testing.Short() {
		t.Skip("live network test; skipped under -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx, &http.Client{Timeout: 15 * time.Second}
}

func TestLivePlatformGetBlockchains(t *testing.T) {
	ctx, _ := liveCtx(t)

	chains, err := platformGetBlockchains(ctx, liveURI)
	if err != nil {
		t.Fatalf("platformGetBlockchains(%s): %v", liveURI, err)
	}
	if len(chains) == 0 {
		t.Fatal("no blockchains returned; the P-chain probe is not reaching the node")
	}
	var haveC bool
	for _, c := range chains {
		t.Logf("  %-8s %s", c.Name, c.ID)
		if c.Name == "C-Chain" {
			haveC = true
		}
	}
	if !haveC {
		t.Fatalf("C-Chain absent from %d blockchains", len(chains))
	}
}

// The failure this whole change exists to prevent: on the old /ext base the
// probe returns an error or an empty list instead of the chain set. Without
// this control a green TestLivePlatformGetBlockchains would prove only that
// the host answers, not that the path matters.
func TestLiveExtBaseIsGone(t *testing.T) {
	ctx, _ := liveCtx(t)

	chains, err := platformGetBlockchains(ctx, liveURI+"/legacy-ext-shim")
	if err == nil && len(chains) > 0 {
		t.Fatalf("bogus base returned %d blockchains; the probe is not path-sensitive", len(chains))
	}
	t.Logf("bogus base correctly yields no chains (err=%v)", err)
}

func TestLiveInfoIsBootstrapped(t *testing.T) {
	ctx, c := liveCtx(t)

	ok, err := infoIsBootstrapped(ctx, c, liveURI, "C")
	if err != nil {
		t.Fatalf("infoIsBootstrapped(C): %v", err)
	}
	if !ok {
		t.Fatal("C-Chain reports not bootstrapped")
	}

	// Negative control: an unknown chain must be reported as not-bootstrapped
	// or as an error, never as true.
	if ok, _ := infoIsBootstrapped(ctx, c, liveURI, "no-such-chain"); ok {
		t.Fatal("unknown chain reported bootstrapped; the probe is not reading the response")
	}
}

func TestLiveEthBlockNumber(t *testing.T) {
	ctx, c := liveCtx(t)

	blk, err := ethBlockNumber(ctx, c, liveURI, "C")
	if err != nil {
		t.Fatalf("ethBlockNumber(C): %v", err)
	}
	if !strings.HasPrefix(blk, "0x") || blk == "0x0" {
		t.Fatalf("eth_blockNumber returned %q; want a non-zero hex height", blk)
	}
	t.Logf("C-Chain height %s", blk)

	if _, err := ethBlockNumber(ctx, c, liveURI, "no-such-chain"); err == nil {
		t.Fatal("unknown chain returned a height; the probe is not path-sensitive")
	}
}
