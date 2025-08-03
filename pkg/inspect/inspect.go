package inspect

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/genesis/pkg/application"
)

// Inspector handles blockchain inspection operations
type Inspector struct {
	app *application.Genesis
}

// New creates a new Inspector instance
func New(app *application.Genesis) *Inspector {
	return &Inspector{app: app}
}

// InspectTip finds and displays the chain tip
func (i *Inspector) InspectTip(dbPath string) error {
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Find the highest block number
	var highestNum uint64
	var highestHash common.Hash

	iter, err := db.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'H' { // Canonical hash prefix
			num := binary.BigEndian.Uint64(key[1:9])
			if num > highestNum {
				highestNum = num
				highestHash = common.BytesToHash(iter.Value())
			}
		}
	}

	if highestNum == 0 {
		fmt.Println("No blocks found in database")
		return nil
	}

	// Get the header for more details
	headerKey := append([]byte("h"), append(highestHash.Bytes(), encodeBlockNumber(highestNum)...)...)
	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		return fmt.Errorf("failed to get header: %w", err)
	}
	defer closer.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}

	fmt.Printf("Chain Tip:\n")
	fmt.Printf("  Block Number: %d\n", highestNum)
	fmt.Printf("  Block Hash:   %s\n", highestHash.Hex())
	fmt.Printf("  Parent Hash:  %s\n", header.ParentHash.Hex())
	fmt.Printf("  State Root:   %s\n", header.Root.Hex())
	fmt.Printf("  Timestamp:    %d\n", header.Time)
	fmt.Printf("  Gas Limit:    %d\n", header.GasLimit)
	fmt.Printf("  Gas Used:     %d\n", header.GasUsed)

	return nil
}

// InspectBlocks displays information about blocks in the database
func (i *Inspector) InspectBlocks(dbPath string, start, count uint64) error {
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Printf("Inspecting blocks from %d (count: %d)\n\n", start, count)

	for num := start; num < start+count; num++ {
		// Get canonical hash
		canonicalKey := append([]byte("H"), encodeBlockNumber(num)...)
		hashData, closer, err := db.Get(canonicalKey)
		if err != nil {
			if num == start {
				return fmt.Errorf("block %d not found", num)
			}
			// No more blocks
			break
		}
		hash := common.BytesToHash(hashData)
		closer.Close()

		// Get header
		headerKey := append([]byte("h"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
		headerData, closer, err := db.Get(headerKey)
		if err != nil {
			fmt.Printf("Block %d: header not found\n", num)
			continue
		}
		closer.Close()

		var header types.Header
		if err := rlp.DecodeBytes(headerData, &header); err != nil {
			fmt.Printf("Block %d: failed to decode header: %v\n", num, err)
			continue
		}

		// Get body
		bodyKey := append([]byte("b"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
		bodyData, closer, err := db.Get(bodyKey)
		if err == nil {
			var body types.Body
			if err := rlp.DecodeBytes(bodyData, &body); err == nil {
				fmt.Printf("Block %d:\n", num)
				fmt.Printf("  Hash:         %s\n", hash.Hex())
				fmt.Printf("  Parent:       %s\n", header.ParentHash.Hex())
				fmt.Printf("  Timestamp:    %d\n", header.Time)
				fmt.Printf("  Transactions: %d\n", len(body.Transactions))
				fmt.Printf("  Gas Used:     %d / %d\n", header.GasUsed, header.GasLimit)
				fmt.Printf("\n")
			}
			closer.Close()
		}
	}

	return nil
}

// InspectKeys shows the different key types in the database
func (i *Inspector) InspectKeys(dbPath string, limit int) error {
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Count different key types
	keyTypes := make(map[string]int)
	var sampleKeys []string

	iter, err := db.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) > 0 {
			prefix := string(key[0])
			keyTypes[prefix]++
			
			if len(sampleKeys) < 10 {
				sampleKeys = append(sampleKeys, fmt.Sprintf("%s: %s", prefix, hex.EncodeToString(key)))
			}
		}
		
		count++
		if limit > 0 && count >= limit {
			break
		}
	}

	// Display results
	fmt.Printf("Key Type Distribution:\n")
	for prefix, cnt := range keyTypes {
		fmt.Printf("  %s: %d keys\n", getKeyDescription(prefix), cnt)
	}
	
	fmt.Printf("\nSample Keys:\n")
	for _, key := range sampleKeys {
		fmt.Printf("  %s\n", key)
	}

	fmt.Printf("\nTotal keys examined: %d\n", count)
	
	return nil
}

// InspectBalance checks the balance of an address at a specific block
func (i *Inspector) InspectBalance(dbPath string, address common.Address, blockNum uint64) error {
	// TODO: This requires implementing state trie access
	return fmt.Errorf("balance inspection not yet implemented")
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func getKeyDescription(prefix string) string {
	switch prefix {
	case "H":
		return "H (canonical hash)"
	case "h":
		return "h (header)"
	case "b":
		return "b (body)"
	case "r":
		return "r (receipt)"
	case "n":
		return "n (number)"
	case "t":
		return "t (transaction)"
	case "R":
		return "R (block receipts)"
	case "l":
		return "l (lookup)"
	default:
		return fmt.Sprintf("%s (unknown)", prefix)
	}
}