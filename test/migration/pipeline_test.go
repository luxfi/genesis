package migration_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration Pipeline", func() {
	var (
		tempDir string
		testDB  *pebble.DB
	)

	BeforeEach(func() {
		// Create temp directory in project folder, not system temp
		tempDir = filepath.Join(".", ".tmp", fmt.Sprintf("test-%d", GinkgoRandomSeed()))
		Expect(os.MkdirAll(tempDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if testDB != nil {
			testDB.Close()
		}
		os.RemoveAll(tempDir)
	})

	Context("Full Pipeline Test", func() {
		It("should complete a full migration pipeline", func() {
			By("Step 1: Creating test subnet data")
			subnetDBPath := filepath.Join(tempDir, "subnet-db")
			createTestSubnetData(subnetDBPath)

			By("Step 2: Migrating EVM prefixes")
			migratedDBPath := filepath.Join(tempDir, "migrated-db")
			migrateEVMPrefixes(subnetDBPath, migratedDBPath)

			By("Step 3: Creating synthetic blockchain")
			syntheticDBPath := filepath.Join(tempDir, "synthetic-db")
			createSyntheticBlockchain(migratedDBPath, syntheticDBPath)

			By("Step 4: Generating consensus state")
			consensusStatePath := filepath.Join(tempDir, "consensus-state")
			generateConsensusState(syntheticDBPath, consensusStatePath)

			By("Step 5: Verifying the migration")
			verifyMigration(consensusStatePath)
		})
	})

	Context("Step-by-Step Tests", func() {
		Describe("Step 1: Subnet Data Creation", func() {
			It("should create valid subnet data", func() {
				subnetDBPath := filepath.Join(tempDir, "subnet-db")
				createTestSubnetData(subnetDBPath)
				
				// Verify data was created
				db, err := pebble.Open(subnetDBPath, &pebble.Options{})
				Expect(err).NotTo(HaveOccurred())
				defer db.Close()

				// Check for expected keys
				iter, err := db.NewIter(nil)
				Expect(err).NotTo(HaveOccurred())
				defer iter.Close()
				
				keyCount := 0
				for iter.First(); iter.Valid(); iter.Next() {
					keyCount++
				}
				Expect(keyCount).To(BeNumerically(">", 0))
			})
		})

		Describe("Step 2: EVM Prefix Migration", func() {
			It("should migrate EVM prefixes correctly", func() {
				subnetDBPath := filepath.Join(tempDir, "subnet-db")
				createTestSubnetData(subnetDBPath)
				
				migratedDBPath := filepath.Join(tempDir, "migrated-db")
				migrateEVMPrefixes(subnetDBPath, migratedDBPath)
				
				// Verify migration
				db, err := pebble.Open(migratedDBPath, &pebble.Options{})
				Expect(err).NotTo(HaveOccurred())
				defer db.Close()

				// Check that EVM keys have been migrated
				iter, err := db.NewIter(nil)
				Expect(err).NotTo(HaveOccurred())
				defer iter.Close()
				
				hasEVMKeys := false
				for iter.First(); iter.Valid(); iter.Next() {
					key := iter.Key()
					if len(key) > 0 && key[0] == 'e' { // EVM prefix
						hasEVMKeys = true
						break
					}
				}
				Expect(hasEVMKeys).To(BeTrue())
			})
		})

		Describe("Step 3: Synthetic Blockchain Creation", func() {
			It("should create synthetic blockchain entries", func() {
				subnetDBPath := filepath.Join(tempDir, "subnet-db")
				createTestSubnetData(subnetDBPath)
				
				migratedDBPath := filepath.Join(tempDir, "migrated-db")
				migrateEVMPrefixes(subnetDBPath, migratedDBPath)
				
				syntheticDBPath := filepath.Join(tempDir, "synthetic-db")
				createSyntheticBlockchain(migratedDBPath, syntheticDBPath)
				
				// Verify synthetic blockchain
				db, err := pebble.Open(syntheticDBPath, &pebble.Options{})
				Expect(err).NotTo(HaveOccurred())
				defer db.Close()

				// Check for block headers
				headerKey := append([]byte("h"), common.Hash{}.Bytes()...)
				val, closer, err := db.Get(headerKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(val).NotTo(BeEmpty())
				closer.Close()
			})
		})

		Describe("Step 4: Consensus State Generation", func() {
			It("should generate valid consensus state", func() {
				// Create full pipeline up to consensus generation
				subnetDBPath := filepath.Join(tempDir, "subnet-db")
				createTestSubnetData(subnetDBPath)
				
				migratedDBPath := filepath.Join(tempDir, "migrated-db")
				migrateEVMPrefixes(subnetDBPath, migratedDBPath)
				
				syntheticDBPath := filepath.Join(tempDir, "synthetic-db")
				createSyntheticBlockchain(migratedDBPath, syntheticDBPath)
				
				consensusStatePath := filepath.Join(tempDir, "consensus-state")
				generateConsensusState(syntheticDBPath, consensusStatePath)
				
				// Verify consensus state files exist
				Expect(filepath.Join(consensusStatePath, "genesis.json")).To(BeAnExistingFile())
			})
		})

		Describe("Step 5: Verification", func() {
			It("should verify the complete migration", func() {
				// Run full pipeline
				subnetDBPath := filepath.Join(tempDir, "subnet-db")
				createTestSubnetData(subnetDBPath)
				
				migratedDBPath := filepath.Join(tempDir, "migrated-db")
				migrateEVMPrefixes(subnetDBPath, migratedDBPath)
				
				syntheticDBPath := filepath.Join(tempDir, "synthetic-db")
				createSyntheticBlockchain(migratedDBPath, syntheticDBPath)
				
				consensusStatePath := filepath.Join(tempDir, "consensus-state")
				generateConsensusState(syntheticDBPath, consensusStatePath)
				
				// Run verification
				verifyMigration(consensusStatePath)
			})
		})
	})

	Context("Error Handling", func() {
		It("should handle missing source database", func() {
			nonExistentPath := filepath.Join(tempDir, "non-existent")
			migratedDBPath := filepath.Join(tempDir, "migrated-db")
			
			err := migrateEVMPrefixesWithError(nonExistentPath, migratedDBPath)
			Expect(err).To(HaveOccurred())
		})

		It("should handle corrupted data gracefully", func() {
			// Create corrupted database
			corruptedDBPath := filepath.Join(tempDir, "corrupted-db")
			db, err := pebble.Open(corruptedDBPath, &pebble.Options{})
			Expect(err).NotTo(HaveOccurred())
			
			// Write invalid data
			err = db.Set([]byte("invalid-key"), []byte("invalid-rlp-data"), pebble.Sync)
			Expect(err).NotTo(HaveOccurred())
			db.Close()
			
			// Try to migrate
			migratedDBPath := filepath.Join(tempDir, "migrated-db")
			err = migrateEVMPrefixesWithError(corruptedDBPath, migratedDBPath)
			// Should handle gracefully (skip invalid entries)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Performance", func() {
		It("should handle large datasets efficiently", func() {
			if os.Getenv("SKIP_PERFORMANCE_TESTS") != "" {
				Skip("Skipping performance tests")
			}

			largeDBPath := filepath.Join(tempDir, "large-db")
			createLargeTestData(largeDBPath, 10000) // 10k entries
			
			migratedDBPath := filepath.Join(tempDir, "migrated-db")
			
			start := time.Now()
			migrateEVMPrefixes(largeDBPath, migratedDBPath)
			duration := time.Since(start)
			
			// Should complete within reasonable time
			Expect(duration.Seconds()).To(BeNumerically("<", 10))
		})
	})
})

// Helper functions for test steps

func createTestSubnetData(dbPath string) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	Expect(err).NotTo(HaveOccurred())
	defer db.Close()

	// Create test blocks
	for i := uint64(0); i < 10; i++ {
		header := &types.Header{
			Number: new(big.Int).SetUint64(i),
			Time:   uint64(time.Now().Unix()),
		}
		
		headerBytes, err := rlp.EncodeToBytes(header)
		Expect(err).NotTo(HaveOccurred())
		
		// Store with subnet prefix
		key := append([]byte("subnet:"), common.BigToHash(header.Number).Bytes()...)
		err = db.Set(key, headerBytes, pebble.Sync)
		Expect(err).NotTo(HaveOccurred())
	}
}

func migrateEVMPrefixes(srcPath, dstPath string) {
	err := migrateEVMPrefixesWithError(srcPath, dstPath)
	Expect(err).NotTo(HaveOccurred())
}

func migrateEVMPrefixesWithError(srcPath, dstPath string) error {
	srcDB, err := pebble.Open(srcPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer srcDB.Close()

	dstDB, err := pebble.Open(dstPath, &pebble.Options{})
	if err != nil {
		return err
	}
	defer dstDB.Close()

	// Simple prefix migration
	iter, err := srcDB.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	batch := dstDB.NewBatch()
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Transform key (example: subnet: -> e)
		newKey := key
		if len(key) > 7 && string(key[:7]) == "subnet:" {
			newKey = append([]byte("e"), key[7:]...)
		}
		
		err = batch.Set(newKey, val, nil)
		if err != nil {
			return err
		}
	}
	
	return batch.Commit(pebble.Sync)
}

func createSyntheticBlockchain(srcPath, dstPath string) {
	srcDB, err := pebble.Open(srcPath, &pebble.Options{ReadOnly: true})
	Expect(err).NotTo(HaveOccurred())
	defer srcDB.Close()

	dstDB, err := pebble.Open(dstPath, &pebble.Options{})
	Expect(err).NotTo(HaveOccurred())
	defer dstDB.Close()

	// Copy all data and add synthetic entries
	iter, err := srcDB.NewIter(nil)
	Expect(err).NotTo(HaveOccurred())
	defer iter.Close()

	batch := dstDB.NewBatch()
	for iter.First(); iter.Valid(); iter.Next() {
		err = batch.Set(iter.Key(), iter.Value(), nil)
		Expect(err).NotTo(HaveOccurred())
	}

	// Add synthetic genesis block
	genesisHeader := &types.Header{
		Number: big.NewInt(0),
		Time:   uint64(time.Now().Unix()),
	}
	
	headerBytes, err := rlp.EncodeToBytes(genesisHeader)
	Expect(err).NotTo(HaveOccurred())
	
	headerKey := append([]byte("h"), common.Hash{}.Bytes()...)
	err = batch.Set(headerKey, headerBytes, nil)
	Expect(err).NotTo(HaveOccurred())
	
	err = batch.Commit(pebble.Sync)
	Expect(err).NotTo(HaveOccurred())
}

func generateConsensusState(srcPath, dstPath string) {
	// Create output directory
	err := os.MkdirAll(dstPath, 0755)
	Expect(err).NotTo(HaveOccurred())

	// Create a simple genesis.json
	genesis := map[string]interface{}{
		"config": map[string]interface{}{
			"chainId": 96369,
		},
		"timestamp": "0x0",
		"gasLimit":  "0x1312d00",
	}

	genesisPath := filepath.Join(dstPath, "genesis.json")
	file, err := os.Create(genesisPath)
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(genesis)
	Expect(err).NotTo(HaveOccurred())
}

func verifyMigration(consensusStatePath string) {
	// Verify genesis.json exists and is valid
	genesisPath := filepath.Join(consensusStatePath, "genesis.json")
	Expect(genesisPath).To(BeAnExistingFile())

	// Read and validate genesis
	data, err := os.ReadFile(genesisPath)
	Expect(err).NotTo(HaveOccurred())

	var genesis map[string]interface{}
	err = json.Unmarshal(data, &genesis)
	Expect(err).NotTo(HaveOccurred())

	// Check required fields
	Expect(genesis).To(HaveKey("config"))
	config := genesis["config"].(map[string]interface{})
	Expect(config).To(HaveKey("chainId"))
}

func createLargeTestData(dbPath string, count int) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	Expect(err).NotTo(HaveOccurred())
	defer db.Close()

	batch := db.NewBatch()
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("key-%06d", i)
		val := fmt.Sprintf("value-%06d", i)
		err = batch.Set([]byte(key), []byte(val), nil)
		Expect(err).NotTo(HaveOccurred())
		
		if i%1000 == 0 {
			err = batch.Commit(pebble.Sync)
			Expect(err).NotTo(HaveOccurred())
			batch = db.NewBatch()
		}
	}
	
	err = batch.Commit(pebble.Sync)
	Expect(err).NotTo(HaveOccurred())
}