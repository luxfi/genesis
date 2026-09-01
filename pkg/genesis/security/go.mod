module github.com/luxfi/genesis/pkg/genesis/security

go 1.26.3

// Security tier — uses luxfi/consensus for ChainSecurityProfile
// verification. Kept SEPARATE so downstream consumers that only
// need genesis data types don't pull luxfi/consensus + its
// post-quantum threshold deps.

require (
	github.com/luxfi/consensus v1.25.1
	github.com/luxfi/genesis v0.0.0-00010101000000-000000000000
)

require (
	github.com/btcsuite/btcd/btcutil v1.1.6 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/gorilla/rpc v1.2.1 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/luxfi/accel v1.1.8 // indirect
	github.com/luxfi/address v1.0.1 // indirect
	github.com/luxfi/cache v1.2.1 // indirect
	github.com/luxfi/codec v1.1.4 // indirect
	github.com/luxfi/constants v1.5.7 // indirect
	github.com/luxfi/container v0.0.4 // indirect
	github.com/luxfi/crypto v1.19.17 // indirect
	github.com/luxfi/geth v1.16.98 // indirect
	github.com/luxfi/go-bip32 v1.0.2 // indirect
	github.com/luxfi/go-bip39 v1.1.2 // indirect
	github.com/luxfi/ids v1.2.10 // indirect
	github.com/luxfi/log v1.4.1 // indirect
	github.com/luxfi/math v1.4.1 // indirect
	github.com/luxfi/math/big v0.1.0 // indirect
	github.com/luxfi/metric v1.5.5 // indirect
	github.com/luxfi/mock v0.1.1 // indirect
	github.com/luxfi/rpc v1.0.2 // indirect
	github.com/luxfi/sampler v1.0.0 // indirect
	github.com/luxfi/tls v1.0.3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/exp v0.0.0-20260312153236-7ab1446f8b90 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
