// rlp-export is a stdlib-only JSON-RPC client for admin_exportChain.
//
// It gets its OWN module rather than living in github.com/luxfi/genesis/cmd
// because it shares none of that module's dependency graph: cmd/ requires
// luxfi/node via `replace => ../../node`, an out-of-tree filesystem path that
// cannot exist inside a container build context. A tool with zero external
// imports must not inherit a 600-line go.sum and an unbuildable replace.
//
// Consequence: Dockerfile.rlp-export builds from this directory alone, with
// no module download and no network.
module github.com/luxfi/genesis/cmd/rlp-export

go 1.26.4
