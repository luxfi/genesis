module github.com/luxfi/genesis/builder

go 1.26.4

// Builder tier — uses luxfi/utxo / vm / database / proto for
// chain-genesis construction. Kept SEPARATE so downstream consumers
// that only need genesis data types (luxd v1.23.x line, indexers)
// don't pull these heavy deps.

require (
	github.com/luxfi/address v1.0.1
	github.com/luxfi/constants v1.5.8-0.20260603055356-93c2c2ceb9ca
	github.com/luxfi/container v0.0.4
	github.com/luxfi/crypto v1.19.17
	github.com/luxfi/formatting v1.0.1
	github.com/luxfi/genesis v1.13.8
	github.com/luxfi/ids v1.2.14
	github.com/luxfi/math v1.4.1
	github.com/luxfi/proto v1.3.0
	github.com/luxfi/utxo v0.3.7
	github.com/luxfi/vm v1.2.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.6 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/gorilla/rpc v1.2.1 // indirect
	github.com/grandcat/zeroconf v1.0.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/luxfi/accel v1.1.9 // indirect
	github.com/luxfi/api v1.0.12 // indirect
	github.com/luxfi/atomic v1.0.0 // indirect
	github.com/luxfi/cache v1.2.1 // indirect
	github.com/luxfi/codec v1.1.5 // indirect
	github.com/luxfi/compress v0.0.5 // indirect
	github.com/luxfi/concurrent v0.0.3 // indirect
	github.com/luxfi/consensus v1.25.13 // indirect
	github.com/luxfi/database v1.18.3 // indirect
	github.com/luxfi/geth v1.16.98 // indirect
	github.com/luxfi/go-bip32 v1.0.2 // indirect
	github.com/luxfi/go-bip39 v1.1.2 // indirect
	github.com/luxfi/keychain v1.0.2 // indirect
	github.com/luxfi/log v1.4.1 // indirect
	github.com/luxfi/math/big v0.1.0 // indirect
	github.com/luxfi/mdns v0.1.1 // indirect
	github.com/luxfi/metric v1.5.7 // indirect
	github.com/luxfi/mock v0.1.1 // indirect
	github.com/luxfi/ordering v0.0.1 // indirect
	github.com/luxfi/p2p v1.21.1 // indirect
	github.com/luxfi/pq v1.0.3 // indirect
	github.com/luxfi/rpc v1.0.2 // indirect
	github.com/luxfi/runtime v1.1.0 // indirect
	github.com/luxfi/sampler v1.1.0 // indirect
	github.com/luxfi/timer v1.0.2 // indirect
	github.com/luxfi/tls v1.0.3 // indirect
	github.com/luxfi/utils v1.1.5 // indirect
	github.com/luxfi/validators v1.2.0 // indirect
	github.com/luxfi/version v1.0.1 // indirect
	github.com/luxfi/warp v1.18.6 // indirect
	github.com/luxfi/zap v0.7.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/exp v0.0.0-20260312153236-7ab1446f8b90 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
