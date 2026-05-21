// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import "github.com/luxfi/address"

// ChainPrefix is the bech32 chain prefix that prefixes the HRP-encoded
// portion of a chain-scoped address. P-Chain UTXOs use "P", X-Chain UTXOs
// use "X". They are orthogonal to the per-network HRP (lux/test/dev/local):
// a prefix identifies the chain, an HRP identifies the network.
//
// Decomplects chain identity from network parameter at the type level —
// callers compose the two by method invocation rather than threading a
// magic string through formatter call sites.
type ChainPrefix string

// Canonical chain prefixes. The X-Chain (UTXO Exchange) and P-Chain
// (ProtocolVM) share the same 20-byte ShortID space; only the prefix
// distinguishes which chain the address binds to.
const (
	PChainPrefix ChainPrefix = "P"
	XChainPrefix ChainPrefix = "X"
)

// Format builds a chain-scoped bech32 address of the form
// "<prefix>-<hrp>1<data><checksum>". The bech32 checksum is computed
// over the HRP and address bytes only — the chain prefix is a textual
// tag joined with "-" after encoding (see luxfi/address.Format).
func (cp ChainPrefix) Format(hrp string, addr []byte) (string, error) {
	return address.Format(string(cp), hrp, addr)
}
