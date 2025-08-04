package genesis_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/genesis"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Genesis Generation", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = filepath.Join(".", ".tmp", fmt.Sprintf("genesis-test-%d", GinkgoRandomSeed()))
		Expect(os.MkdirAll(tempDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("P-Chain Genesis", func() {
		It("should generate valid P-Chain genesis", func() {
			validators := []genesis.Validator{
				{
					NodeID:     "NodeID-111111111111111111116DBWJs",
					Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
					Weight:     1000000,
				},
			}

			pGenesis := genesis.CreatePChainGenesis(validators, "mainnet")
			Expect(pGenesis).NotTo(BeNil())

			// Verify structure
			genesisData, err := json.Marshal(pGenesis)
			Expect(err).NotTo(HaveOccurred())

			var genesisMap map[string]interface{}
			err = json.Unmarshal(genesisData, &genesisMap)
			Expect(err).NotTo(HaveOccurred())

			// Check required fields
			Expect(genesisMap).To(HaveKey("allocations"))
			Expect(genesisMap).To(HaveKey("initialStakeDuration"))
			Expect(genesisMap).To(HaveKey("initialStakers"))
		})

		It("should handle multiple validators", func() {
			validators := []genesis.Validator{}
			for i := 0; i < 10; i++ {
				validator, err := genesis.GenerateValidator()
				Expect(err).NotTo(HaveOccurred())
				validators = append(validators, validator)
			}

			pGenesis := genesis.CreatePChainGenesis(validators, "mainnet")
			Expect(pGenesis).NotTo(BeNil())

			// Write to file
			outputPath := filepath.Join(tempDir, "p-genesis.json")
			err := genesis.WritePChainGenesis(pGenesis, outputPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputPath).To(BeAnExistingFile())
		})
	})

	Context("C-Chain Genesis", func() {
		It("should generate valid C-Chain genesis", func() {
			allocations := map[string]string{
				"0x0100000000000000000000000000000000000000": "100000000000000000000000",
				"0x0200000000000000000000000000000000000000": "200000000000000000000000",
			}

			cGenesis := genesis.CreateCChainGenesis(allocations, "mainnet")
			Expect(cGenesis).NotTo(BeNil())

			// Verify structure
			genesisData, err := json.Marshal(cGenesis)
			Expect(err).NotTo(HaveOccurred())

			var genesisMap map[string]interface{}
			err = json.Unmarshal(genesisData, &genesisMap)
			Expect(err).NotTo(HaveOccurred())

			// Check required fields
			Expect(genesisMap).To(HaveKey("config"))
			Expect(genesisMap).To(HaveKey("alloc"))
			Expect(genesisMap).To(HaveKey("difficulty"))
			Expect(genesisMap).To(HaveKey("gasLimit"))

			// Verify config
			config := genesisMap["config"].(map[string]interface{})
			Expect(config).To(HaveKey("chainId"))
			Expect(config["chainId"]).To(Equal(float64(96369))) // mainnet chain ID
		})

		It("should include fee manager addresses", func() {
			allocations := map[string]string{}
			cGenesis := genesis.CreateCChainGenesis(allocations, "mainnet")
			
			genesisData, err := json.Marshal(cGenesis)
			Expect(err).NotTo(HaveOccurred())

			// Check fee manager address is included
			Expect(string(genesisData)).To(ContainSubstring("0x0100000000000000000000000000000000000003"))
		})
	})

	Context("X-Chain Genesis", func() {
		It("should generate valid X-Chain genesis", func() {
			xGenesis := genesis.CreateXChainGenesis("mainnet")
			Expect(xGenesis).NotTo(BeNil())

			// Verify structure
			genesisData, err := json.Marshal(xGenesis)
			Expect(err).NotTo(HaveOccurred())

			var genesisMap map[string]interface{}
			err = json.Unmarshal(genesisData, &genesisMap)
			Expect(err).NotTo(HaveOccurred())

			// Check required fields
			Expect(genesisMap).To(HaveKey("allocations"))
			Expect(genesisMap).To(HaveKey("initialSupply"))
			Expect(genesisMap).To(HaveKey("networkID"))
		})
	})

	Context("Combined Genesis", func() {
		It("should generate all three genesis files", func() {
			outputDir := filepath.Join(tempDir, "lux-mainnet-96369")
			
			// Create validators
			validators := []genesis.Validator{
				{
					NodeID:     "NodeID-111111111111111111116DBWJs",
					Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
					Weight:     1000000,
				},
			}

			// Create allocations
			allocations := map[string]string{
				"0x0100000000000000000000000000000000000000": "100000000000000000000000",
			}

			// Generate all genesis files
			err := genesis.GenerateAllForTest(outputDir, validators, allocations, "mainnet")
			Expect(err).NotTo(HaveOccurred())

			// Verify all files exist
			Expect(filepath.Join(outputDir, "P", "genesis.json")).To(BeAnExistingFile())
			Expect(filepath.Join(outputDir, "C", "genesis.json")).To(BeAnExistingFile())
			Expect(filepath.Join(outputDir, "X", "genesis.json")).To(BeAnExistingFile())
		})

		It("should handle testnet configuration", func() {
			outputDir := filepath.Join(tempDir, "lux-testnet-96368")
			
			validators := []genesis.Validator{}
			allocations := map[string]string{}

			err := genesis.GenerateAllForTest(outputDir, validators, allocations, "testnet")
			Expect(err).NotTo(HaveOccurred())

			// Read C-Chain genesis to verify testnet chain ID
			cGenesisPath := filepath.Join(outputDir, "C", "genesis.json")
			data, err := os.ReadFile(cGenesisPath)
			Expect(err).NotTo(HaveOccurred())

			var cGenesis map[string]interface{}
			err = json.Unmarshal(data, &cGenesis)
			Expect(err).NotTo(HaveOccurred())

			config := cGenesis["config"].(map[string]interface{})
			Expect(config["chainId"]).To(Equal(float64(96368))) // testnet chain ID
		})
	})

	Context("Configuration Files", func() {
		It("should generate validator configuration", func() {
			validators := []genesis.Validator{
				{
					NodeID:     "NodeID-111111111111111111116DBWJs",
					Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
					Weight:     1000000,
				},
			}

			outputPath := filepath.Join(tempDir, "validators.json")
			err := genesis.WriteValidatorConfig(validators, outputPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputPath).To(BeAnExistingFile())

			// Read and verify
			data, err := os.ReadFile(outputPath)
			Expect(err).NotTo(HaveOccurred())

			var readValidators []genesis.Validator
			err = json.Unmarshal(data, &readValidators)
			Expect(err).NotTo(HaveOccurred())
			Expect(readValidators).To(HaveLen(1))
			Expect(readValidators[0].NodeID).To(Equal(validators[0].NodeID))
		})

		It("should generate address list", func() {
			allocations := map[string]string{
				"0x0100000000000000000000000000000000000000": "100000000000000000000000",
				"0x0200000000000000000000000000000000000000": "200000000000000000000000",
			}

			outputPath := filepath.Join(tempDir, "addresses.json")
			err := genesis.WriteAddressList(allocations, outputPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputPath).To(BeAnExistingFile())
		})
	})

	Context("Network Parameters", func() {
		It("should use correct parameters for mainnet", func() {
			params := genesis.GetNetworkParams("mainnet")
			Expect(params.ChainID).To(Equal(uint64(96369)))
			Expect(params.NetworkID).To(Equal(uint32(96369)))
		})

		It("should use correct parameters for testnet", func() {
			params := genesis.GetNetworkParams("testnet")
			Expect(params.ChainID).To(Equal(uint64(96368)))
			Expect(params.NetworkID).To(Equal(uint32(96368)))
		})

		It("should default to mainnet for unknown network", func() {
			params := genesis.GetNetworkParams("unknown")
			Expect(params.ChainID).To(Equal(uint64(96369)))
		})
	})
})