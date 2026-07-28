// rlp-import is a stdlib-only JSON-RPC client for admin_importChain —
// symmetric with cmd/rlp-export, and carved out of github.com/luxfi/genesis/cmd
// for the same reason: it imports nothing outside the standard library, so it
// must not inherit that module's `replace github.com/luxfi/node => ../../node`,
// which is unresolvable in a container build context.
module github.com/luxfi/genesis/cmd/rlp-import

go 1.26.4
