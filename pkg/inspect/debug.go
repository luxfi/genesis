package inspect

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/luxfi/database"
)

// DebugKeys shows detailed information about database keys
func (i *Inspector) DebugKeys(dbPath string, prefix string, limit int) error {
	db, err := i.openDatabase(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create iterator with optional prefix
	var iter database.Iterator
	if prefix != "" {
		iter = db.NewIteratorWithPrefix([]byte(prefix))
	} else {
		iter = db.NewIterator()
	}
	defer iter.Release()

	// Count different key types
	keyTypes := make(map[string]int)
	canonicalCount := 0
	headerCount := 0

	count := 0
	for iter.Next() && count < limit {
		key := iter.Key()
		value := iter.Value()

		if len(key) == 0 {
			continue
		}

		// Analyze key structure
		keyStr := string(key[0])
		keyTypes[keyStr]++

		// Show detailed info for interesting keys
		switch key[0] {
		case 'h': // Could be header or canonical hash
			if len(key) == 10 && key[9] == 'n' {
				// Canonical hash: h + num(8) + n
				blockNum := binary.BigEndian.Uint64(key[1:9])
				fmt.Printf("Canonical: block %d -> hash %s\n", blockNum, hex.EncodeToString(value))
				canonicalCount++
			} else if len(key) == 41 {
				// Header: h + num(8) + hash(32)
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Header: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
				headerCount++
			} else {
				fmt.Printf("Unknown 'h' key: len=%d, hex=%s\n", len(key), hex.EncodeToString(key))
			}

		case 'H': // Hash to number mapping
			if len(key) == 33 {
				hash := hex.EncodeToString(key[1:33])
				if len(value) == 8 {
					blockNum := binary.BigEndian.Uint64(value)
					fmt.Printf("HashToNum: %s -> block %d\n", hash, blockNum)
				}
			}

		case 'b': // Block body
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Body: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
			}

		case 'r': // Receipts
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Receipts: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
			}
		}

		count++
	}

	fmt.Printf("\nKey type summary:\n")
	for k, v := range keyTypes {
		fmt.Printf("  '%s': %d keys\n", k, v)
	}

	fmt.Printf("\nFound %d canonical mappings\n", canonicalCount)
	fmt.Printf("Found %d headers\n", headerCount)

	return nil
}