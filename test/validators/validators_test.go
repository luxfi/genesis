package validators_test

import (
	"encoding/json"

	"github.com/luxfi/genesis/pkg/genesis"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validators", func() {
	Context("Validator Creation", func() {
		It("should create a validator with valid NodeID", func() {
			validator := genesis.Validator{
				NodeID:     "NodeID-111111111111111111116DBWJs",
				Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
				Weight:     1000000,
			}

			Expect(validator.NodeID).To(HavePrefix("NodeID-"))
			Expect(validator.Bech32Addr).To(HavePrefix("lux"))
			Expect(validator.Weight).To(BeNumerically(">", 0))
		})

		It("should generate validator from private key", func() {
			// Test generating validator from secp256k1 key
			validator, err := genesis.GenerateValidator()
			Expect(err).NotTo(HaveOccurred())

			Expect(validator.NodeID).To(HavePrefix("NodeID-"))
			Expect(validator.Bech32Addr).To(HavePrefix("lux"))
			Expect(validator.Weight).To(Equal(uint64(1000000))) // Default weight
		})
	})

	Context("Validator List Management", func() {
		var validatorList []genesis.Validator

		BeforeEach(func() {
			validatorList = []genesis.Validator{}
		})

		It("should add validators to list", func() {
			for i := 0; i < 5; i++ {
				validator, err := genesis.GenerateValidator()
				Expect(err).NotTo(HaveOccurred())
				validatorList = append(validatorList, validator)
			}

			Expect(validatorList).To(HaveLen(5))

			// Check all validators are unique
			nodeIDs := make(map[string]bool)
			for _, v := range validatorList {
				Expect(nodeIDs[v.NodeID]).To(BeFalse())
				nodeIDs[v.NodeID] = true
			}
		})

		It("should calculate total stake correctly", func() {
			validators := []genesis.Validator{
				{NodeID: "NodeID-1", Bech32Addr: "lux1a", Weight: 1000000},
				{NodeID: "NodeID-2", Bech32Addr: "lux1b", Weight: 2000000},
				{NodeID: "NodeID-3", Bech32Addr: "lux1c", Weight: 3000000},
			}

			totalStake := uint64(0)
			for _, v := range validators {
				totalStake += v.Weight
			}

			Expect(totalStake).To(Equal(uint64(6000000)))
		})
	})

	Context("Validator Serialization", func() {
		It("should serialize validators to JSON", func() {
			validators := []genesis.Validator{
				{
					NodeID:     "NodeID-111111111111111111116DBWJs",
					Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
					Weight:     1000000,
				},
			}

			data, err := json.Marshal(validators)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("NodeID-"))
			Expect(string(data)).To(ContainSubstring("lux1"))
		})

		It("should deserialize validators from JSON", func() {
			jsonData := `[{
				"nodeID": "NodeID-111111111111111111116DBWJs",
				"bech32Addr": "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
				"weight": 1000000
			}]`

			var validators []genesis.Validator
			err := json.Unmarshal([]byte(jsonData), &validators)
			Expect(err).NotTo(HaveOccurred())

			Expect(validators).To(HaveLen(1))
			Expect(validators[0].NodeID).To(Equal("NodeID-111111111111111111116DBWJs"))
			Expect(validators[0].Weight).To(Equal(uint64(1000000)))
		})
	})

	Context("Validator Validation", func() {
		It("should validate NodeID format", func() {
			validNodeID := "NodeID-111111111111111111116DBWJs"
			Expect(genesis.IsValidNodeID(validNodeID)).To(BeTrue())

			invalidNodeID := "InvalidNodeID"
			Expect(genesis.IsValidNodeID(invalidNodeID)).To(BeFalse())
		})

		It("should validate Bech32 address format", func() {
			validAddr := "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla"
			Expect(genesis.IsValidBech32Addr(validAddr)).To(BeTrue())

			invalidAddr := "invalid-address"
			Expect(genesis.IsValidBech32Addr(invalidAddr)).To(BeFalse())
		})

		It("should reject zero weight validators", func() {
			validator := genesis.Validator{
				NodeID:     "NodeID-111111111111111111116DBWJs",
				Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
				Weight:     0,
			}

			err := genesis.ValidateValidator(validator)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("weight"))
		})
	})

	Context("Integration with Genesis", func() {
		It("should add validators to P-Chain genesis", func() {
			validators := []genesis.Validator{
				{
					NodeID:     "NodeID-111111111111111111116DBWJs",
					Bech32Addr: "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla",
					Weight:     1000000,
				},
			}

			pGenesis := genesis.CreatePChainGenesis(validators, "mainnet")
			Expect(pGenesis).NotTo(BeNil())

			// Check that validators are included
			genesisData, err := json.Marshal(pGenesis)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(genesisData)).To(ContainSubstring("NodeID-"))
		})
	})
})
