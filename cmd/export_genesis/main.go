package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// GenesisExport represents the exported genesis data
type GenesisExport struct {
	NetworkID   int                    `json:"networkId"`
	NetworkName string                 `json:"networkName"`
	ChainID     int64                  `json:"chainId"`
	Block0      *GenesisBlock          `json:"block0,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// GenesisBlock represents the genesis block data
type GenesisBlock struct {
	Hash       string `json:"hash"`
	Number     uint64 `json:"number"`
	Timestamp  uint64 `json:"timestamp"`
	GasLimit   uint64 `json:"gasLimit"`
	Difficulty string `json:"difficulty"`
	ExtraData  string `json:"extraData"`
	Coinbase   string `json:"coinbase"`
}

var networks = map[string]struct {
	name      string
	path      string
	networkID int
	chainID   int64
	dbType    string
	namespace []byte
}{
	"lux-mainnet": {
		name:      "Lux Mainnet",
		path:      "state/chaindata/lux-mainnet-96369/db",
		networkID: 96369,
		chainID:   96369,
		dbType:    "pebbledb",
		namespace: []byte{0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1, 0x00, 0x00},
	},
	"lux-testnet": {
		name:      "Lux Testnet",
		path:      "state/chaindata/lux-testnet-96368/db",
		networkID: 96368,
		chainID:   96368,
		dbType:    "pebbledb",
		namespace: nil, // Will need to determine
	},
	"lux-genesis": {
		name:      "Lux Genesis",
		path:      "state/chaindata/lux-genesis-7777/db",
		networkID: 7777,
		chainID:   7777,
		dbType:    "leveldb",
		namespace: nil,
	},
	"zoo-mainnet": {
		name:      "Zoo Mainnet",
		path:      "state/chaindata/zoo-mainnet-200200/db",
		networkID: 200200,
		chainID:   200200,
		dbType:    "pebbledb",
		namespace: nil, // Will need to determine
	},
	"zoo-testnet": {
		name:      "Zoo Testnet",
		path:      "state/chaindata/zoo-testnet-200201/db",
		networkID: 200201,
		chainID:   200201,
		dbType:    "pebbledb",
		namespace: nil, // Will need to determine
	},
	"spc-mainnet": {
		name:      "SPC Mainnet",
		path:      "state/chaindata/spc-mainnet-36911/db",
		networkID: 36911,
		chainID:   36911,
		dbType:    "pebbledb",
		namespace: nil, // Will need to determine
	},
}

var rootCmd = &cobra.Command{
	Use:   "export-genesis",
	Short: "Export genesis data from various network databases",
	Long:  "Extract genesis block and configuration from Lux, Zoo, and SPC network databases",
}

var exportCmd = &cobra.Command{
	Use:   "export [network]",
	Short: "Export genesis for a specific network",
	Args:  cobra.ExactArgs(1),
	Run:   runExport,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available networks",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available networks:")
		for key, net := range networks {
			fmt.Printf("  %s: %s (Network ID: %d, Chain ID: %d)\n", 
				key, net.name, net.networkID, net.chainID)
		}
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(listCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExport(cmd *cobra.Command, args []string) {
	networkKey := args[0]
	network, exists := networks[networkKey]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown network: %s\n", networkKey)
		fmt.Println("Use 'export-genesis list' to see available networks")
		os.Exit(1)
	}

	export := &GenesisExport{
		NetworkID:   network.networkID,
		NetworkName: network.name,
		ChainID:     network.chainID,
	}

	dbPath := filepath.Join(os.Getenv("HOME"), "work/lux/genesis", network.path)
	
	// Handle different database types
	if network.dbType == "leveldb" {
		// Open LevelDB
		ldb, err := leveldb.OpenFile(dbPath, &opt.Options{
			ReadOnly: true,
		})
		if err != nil {
			export.Error = fmt.Sprintf("Failed to open LevelDB: %v", err)
			outputExport(export)
			os.Exit(1)
		}
		defer ldb.Close()
		
		// If we don't have a namespace, try to detect it
		if network.namespace == nil {
			namespace := detectNamespaceLevelDB(ldb)
			if namespace != nil {
				fmt.Printf("Detected namespace: %x\n", namespace)
				network.namespace = namespace
			}
		}
		
		// Try to find the genesis block (block 0)
		block0 := findGenesisBlockLevelDB(ldb, network.namespace)
		if block0 != nil {
			export.Block0 = block0
		}
		
		// Also try to extract state from the genesis
		export.Config = extractGenesisStateLevelDB(ldb, network.namespace)
	} else {
		// Open PebbleDB
		pdbPath := dbPath + "/pebbledb"
		
		// Check if pebbledb directory exists
		if _, err := os.Stat(pdbPath); os.IsNotExist(err) {
			export.Error = fmt.Sprintf("PebbleDB directory does not exist: %s", pdbPath)
			outputExport(export)
			return
		}
		
		// Remove LOCK file if it exists (for read-only access)
		lockFile := filepath.Join(pdbPath, "LOCK")
		os.Remove(lockFile) // Ignore error if it doesn't exist
		
		pdb, err := pebble.Open(pdbPath, &pebble.Options{
			ReadOnly: true,
		})
		if err != nil {
			export.Error = fmt.Sprintf("Failed to open PebbleDB: %v", err)
			outputExport(export)
			return
		}
		defer pdb.Close()
		
		// If we don't have a namespace, try to detect it
		if network.namespace == nil {
			namespace := detectNamespace(pdb)
			if namespace != nil {
				fmt.Printf("Detected namespace: %x\n", namespace)
				network.namespace = namespace
			}
		}
		
		// Try to find the genesis block (block 0)
		block0 := findGenesisBlock(pdb, network.namespace)
		if block0 != nil {
			export.Block0 = block0
		}
		
		// Also try to extract state from the genesis
		export.Config = extractGenesisStatePebble(pdb, network.namespace)
	}

	// Output the export
	outputExport(export)
}

func detectNamespace(db *pebble.DB) []byte {
	// Try to find a header key pattern
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil
	}
	defer iter.Close()

	// Look for keys that might be headers (start with 'h' after namespace)
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) > 35 { // Namespace (32-34 bytes) + 'h' + data
			// Check if this looks like a header key
			// Headers have pattern: namespace + 'h' + 8 bytes block num + 32 bytes hash
			possibleHeaderMarker := key[len(key)-43]
			if possibleHeaderMarker == 'h' {
				// This might be a header, extract namespace
				namespace := key[:len(key)-43]
				fmt.Printf("Possible namespace detected (length %d)\n", len(namespace))
				return namespace
			}
		}
	}
	return nil
}

func findGenesisBlock(db *pebble.DB, namespace []byte) *GenesisBlock {
	// Try to find block 0
	// Key format: namespace + 'h' + blockNum(8 bytes) + hash(32 bytes)
	
	// First, try to find the canonical hash for block 0
	// Canonical key: namespace + 'h' + blockNum(8 bytes) + 'n'
	canonicalKey := append(namespace, 'h')
	canonicalKey = append(canonicalKey, make([]byte, 8)...) // block 0
	canonicalKey = append(canonicalKey, 'n')
	
	canonicalValue, closer, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("No canonical hash found for block 0\n")
		return nil
	}
	defer closer.Close()
	
	if len(canonicalValue) != 32 {
		fmt.Printf("Invalid canonical hash length: %d\n", len(canonicalValue))
		return nil
	}
	
	var hash common.Hash
	copy(hash[:], canonicalValue)
	
	// Now get the header using the hash
	headerKey := append(namespace, 'h')
	headerKey = append(headerKey, make([]byte, 8)...) // block 0
	headerKey = append(headerKey, hash[:]...)
	
	headerValue, closer2, err := db.Get(headerKey)
	if err != nil {
		fmt.Printf("No header found for block 0 with hash %x\n", hash)
		return nil
	}
	defer closer2.Close()
	
	// Decode the RLP-encoded header
	var header types.Header
	if err := rlp.DecodeBytes(headerValue, &header); err != nil {
		fmt.Printf("Failed to decode header: %v\n", err)
		return nil
	}
	
	return &GenesisBlock{
		Hash:       hash.Hex(),
		Number:     header.Number.Uint64(),
		Timestamp:  header.Time,
		GasLimit:   header.GasLimit,
		Difficulty: header.Difficulty.String(),
		ExtraData:  common.Bytes2Hex(header.Extra),
		Coinbase:   header.Coinbase.Hex(),
	}
}

func detectNamespaceLevelDB(ldb *leveldb.DB) []byte {
	// Try to find a header key pattern
	iter := ldb.NewIterator(nil, nil)
	defer iter.Release()

	// Look for keys that might be headers (start with 'h' after namespace)
	for iter.Next() {
		key := iter.Key()
		if len(key) > 35 { // Namespace (32-34 bytes) + 'h' + data
			// Check if this looks like a header key
			// Headers have pattern: namespace + 'h' + 8 bytes block num + 32 bytes hash
			for nsLen := 32; nsLen <= 34; nsLen++ {
				if len(key) > nsLen && key[nsLen] == 'h' && len(key) == nsLen+1+8+32 {
					// This looks like a header key
					namespace := make([]byte, nsLen)
					copy(namespace, key[:nsLen])
					fmt.Printf("Possible namespace detected (length %d): %x\n", len(namespace), namespace)
					return namespace
				}
			}
		}
	}
	return nil
}

func findGenesisBlockLevelDB(ldb *leveldb.DB, namespace []byte) *GenesisBlock {
	// Try to find block 0
	// Key format: namespace + 'h' + blockNum(8 bytes) + hash(32 bytes)
	
	// First, try to find the canonical hash for block 0
	// Canonical key: namespace + 'h' + blockNum(8 bytes) + 'n'
	var canonicalKey []byte
	if namespace != nil {
		canonicalKey = append(namespace, 'h')
	} else {
		canonicalKey = []byte{'h'}
	}
	canonicalKey = append(canonicalKey, make([]byte, 8)...) // block 0
	canonicalKey = append(canonicalKey, 'n')
	
	canonicalValue, err := ldb.Get(canonicalKey, nil)
	if err != nil {
		fmt.Printf("No canonical hash found for block 0: %v\n", err)
		// Try without namespace
		if namespace != nil {
			fmt.Println("Trying without namespace...")
			return findGenesisBlockLevelDB(ldb, nil)
		}
		return nil
	}
	
	if len(canonicalValue) != 32 {
		fmt.Printf("Invalid canonical hash length: %d\n", len(canonicalValue))
		return nil
	}
	
	var hash common.Hash
	copy(hash[:], canonicalValue)
	fmt.Printf("Found canonical hash for block 0: %x\n", hash)
	
	// Now get the header using the hash
	var headerKey []byte
	if namespace != nil {
		headerKey = append(namespace, 'h')
	} else {
		headerKey = []byte{'h'}
	}
	headerKey = append(headerKey, make([]byte, 8)...) // block 0
	headerKey = append(headerKey, hash[:]...)
	
	headerValue, err := ldb.Get(headerKey, nil)
	if err != nil {
		fmt.Printf("No header found for block 0 with hash %x: %v\n", hash, err)
		return nil
	}
	
	// Decode the RLP-encoded header
	var header types.Header
	if err := rlp.DecodeBytes(headerValue, &header); err != nil {
		fmt.Printf("Failed to decode header: %v\n", err)
		return nil
	}
	
	fmt.Printf("Successfully decoded genesis block header\n")
	
	return &GenesisBlock{
		Hash:       hash.Hex(),
		Number:     header.Number.Uint64(),
		Timestamp:  header.Time,
		GasLimit:   header.GasLimit,
		Difficulty: header.Difficulty.String(),
		ExtraData:  common.Bytes2Hex(header.Extra),
		Coinbase:   header.Coinbase.Hex(),
	}
}

func extractGenesisStateLevelDB(ldb *leveldb.DB, namespace []byte) map[string]interface{} {
	config := make(map[string]interface{})
	
	// Try to find accounts in the state
	accounts := make(map[string]interface{})
	iter := ldb.NewIterator(nil, nil)
	defer iter.Release()
	
	for iter.Next() {
		key := iter.Key()
		// Look for account data
		if len(key) > 0 && (key[len(key)-1] == 'a' || key[len(key)-1] == 'b') {
			// Might be account balance or account data
			if len(key) == 33 || (namespace != nil && len(key) == len(namespace)+33) {
				// This could be an account
				value := iter.Value()
				if len(value) > 0 {
					// Try to decode as account
					fmt.Printf("Possible account key: %x\n", key)
				}
			}
		}
		
		// Look for chain config
		if bytes.Contains(key, []byte("ethereum-config-")) {
			value := iter.Value()
			var chainConfig map[string]interface{}
			if err := json.Unmarshal(value, &chainConfig); err == nil {
				config["chainConfig"] = chainConfig
				fmt.Println("Found chain config")
			}
		}
		
		// Look for LastAccepted or LastBlock
		if bytes.Contains(key, []byte("LastAccepted")) || bytes.Contains(key, []byte("LastBlock")) {
			fmt.Printf("Found key: %s = %x\n", key, iter.Value())
		}
	}
	
	if len(accounts) > 0 {
		config["accounts"] = accounts
	}
	
	return config
}

func extractGenesisStatePebble(pdb *pebble.DB, namespace []byte) map[string]interface{} {
	config := make(map[string]interface{})
	
	// Try to find accounts in the state
	accounts := make(map[string]interface{})
	iter, err := pdb.NewIter(&pebble.IterOptions{})
	if err != nil {
		return config
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		// Look for account data
		if len(key) > 0 && (key[len(key)-1] == 'a' || key[len(key)-1] == 'b') {
			// Might be account balance or account data
			if len(key) == 33 || (namespace != nil && len(key) == len(namespace)+33) {
				// This could be an account
				value, err := iter.ValueAndErr()
				if err == nil && len(value) > 0 {
					// Try to decode as account
					fmt.Printf("Possible account key: %x\n", key)
				}
			}
		}
		
		// Look for chain config
		if bytes.Contains(key, []byte("ethereum-config-")) {
			value, err := iter.ValueAndErr()
			if err == nil {
				var chainConfig map[string]interface{}
				if err := json.Unmarshal(value, &chainConfig); err == nil {
					config["chainConfig"] = chainConfig
					fmt.Println("Found chain config")
				}
			}
		}
		
		// Look for LastAccepted or LastBlock
		if bytes.Contains(key, []byte("LastAccepted")) || bytes.Contains(key, []byte("LastBlock")) {
			value, _ := iter.ValueAndErr()
			fmt.Printf("Found key: %s = %x\n", key, value)
		}
	}
	
	if len(accounts) > 0 {
		config["accounts"] = accounts
	}
	
	return config
}

func outputExport(export *GenesisExport) {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal export: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(string(data))
	
	// Also save to file
	filename := fmt.Sprintf("genesis_export_%d.json", export.NetworkID)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write file: %v\n", err)
	} else {
		fmt.Printf("\nExport saved to: %s\n", filename)
	}
}