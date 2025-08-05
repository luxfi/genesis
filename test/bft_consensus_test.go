package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("BFT Consensus with K=1", func() {
	var (
		testDir     string
		genesisPath string
		session     *gexec.Session
	)

	BeforeEach(func() {
		// Create test directory
		testDir = filepath.Join(".tmp", fmt.Sprintf("bft-test-%d", time.Now().Unix()))
		Expect(os.MkdirAll(testDir, 0755)).To(Succeed())
		
		// Build genesis binary
		genesisPath = filepath.Join(testDir, "genesis")
		genesisCmd := exec.Command("go", "build", "-o", genesisPath, "./cmd/genesis")
		genesisCmd.Dir = ".."
		Expect(genesisCmd.Run()).To(Succeed())
	})

	AfterEach(func() {
		if session != nil {
			session.Terminate().Wait()
		}
		os.RemoveAll(testDir)
	})

	Context("BLS Key Generation", func() {
		It("should generate valid BLS keys with proof of possession", func() {
			stakingDir := filepath.Join(testDir, "staking")
			
			// Generate keys
			cmd := exec.Command(filepath.Join(testDir, "genesis"), "staking", "keygen", "--output", stakingDir)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			
			Eventually(session).Should(gexec.Exit(0))
			
			// Verify output
			Expect(session.Out).To(gbytes.Say("Generated Staking Keys"))
			Expect(session.Out).To(gbytes.Say("BLS Public Key: 0x"))
			Expect(session.Out).To(gbytes.Say("BLS Proof of Possession: 0x"))
			
			// Check files exist
			Expect(filepath.Join(stakingDir, "staker.crt")).To(BeAnExistingFile())
			Expect(filepath.Join(stakingDir, "staker.key")).To(BeAnExistingFile())
			Expect(filepath.Join(stakingDir, "signer.key")).To(BeAnExistingFile())
			Expect(filepath.Join(stakingDir, "genesis-staker.json")).To(BeAnExistingFile())
			
			// Verify genesis staker JSON
			data, err := os.ReadFile(filepath.Join(stakingDir, "genesis-staker.json"))
			Expect(err).NotTo(HaveOccurred())
			
			var staker map[string]interface{}
			Expect(json.Unmarshal(data, &staker)).To(Succeed())
			
			Expect(staker).To(HaveKey("nodeID"))
			Expect(staker).To(HaveKey("signer"))
			
			signer := staker["signer"].(map[string]interface{})
			Expect(signer).To(HaveKey("publicKey"))
			Expect(signer).To(HaveKey("proofOfPossession"))
		})
	})

	Context("Genesis Creation", func() {
		It("should create genesis with single BLS validator", func() {
			// First generate keys
			stakingDir := filepath.Join(testDir, "staking")
			cmd := exec.Command(filepath.Join(testDir, "genesis"), "staking", "keygen", "--output", stakingDir)
			Expect(cmd.Run()).To(Succeed())
			
			// Read staker info
			data, err := os.ReadFile(filepath.Join(stakingDir, "genesis-staker.json"))
			Expect(err).NotTo(HaveOccurred())
			
			var staker map[string]interface{}
			Expect(json.Unmarshal(data, &staker)).To(Succeed())
			
			// Create genesis
			genesisPath = filepath.Join(testDir, "genesis.json")
			genesis := map[string]interface{}{
				"networkID": 96369,
				"allocations": []map[string]interface{}{
					{
						"ethAddr":       "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
						"luxAddr":       "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
						"initialAmount": 300000000000000000,
						"unlockSchedule": []interface{}{},
					},
				},
				"startTime":                  time.Now().Unix(),
				"initialStakeDuration":       31536000,
				"initialStakeDurationOffset": 5400,
				"initialStakedFunds":         []string{"X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"},
				"initialStakers":             []interface{}{staker},
				"cChainGenesis":              `{"config":{"chainId":96369},"alloc":{"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC":{"balance":"0x295BE96E64066972000000"}}}`,
				"message":                    "BFT K=1 Test Genesis",
			}
			
			genesisData, err := json.MarshalIndent(genesis, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(genesisPath, genesisData, 0644)).To(Succeed())
		})
	})

	Context("Node Launch", func() {
		It("should launch node with k=1 consensus", func() {
			Skip("Requires luxd binary - tested in integration")
			
			// This would launch the actual node
			// In CI, we test that the configuration is valid
			// and would be accepted by luxd
		})
	})

	Context("Consensus Parameters", func() {
		It("should validate k=1 parameters", func() {
			// Test parameter validation through genesis tool
			cmd := exec.Command(filepath.Join(testDir, "genesis"), "consensus", "validate",
				"--k", "1",
				"--alpha-preference", "1",
				"--alpha-confidence", "1",
				"--beta", "1",
			)
			
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("Consensus parameters are valid"))
		})

		It("should reject invalid k=1 parameters", func() {
			// Alpha preference must be > k/2
			cmd := exec.Command(filepath.Join(testDir, "genesis"), "consensus", "validate",
				"--k", "1",
				"--alpha-preference", "0", // Invalid: must be > 0.5
				"--alpha-confidence", "1",
				"--beta", "1",
			)
			
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("k/2 < alphaPreference"))
		})
	})
})

var _ = Describe("BFT Integration Test", func() {
	var testDir string

	BeforeEach(func() {
		testDir = filepath.Join(".tmp", fmt.Sprintf("bft-integration-%d", time.Now().Unix()))
		Expect(os.MkdirAll(testDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	It("should complete full BFT setup workflow", func() {
		// Build genesis tool
		buildCmd := exec.Command("make", "build")
		buildCmd.Dir = ".."
		output, err := buildCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		
		// Generate BLS keys
		keygenCmd := exec.Command("../bin/genesis", "staking", "keygen", "--output", filepath.Join(testDir, "staking"))
		output, err = keygenCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(output).To(ContainSubstring("All staking keys generated successfully"))
		
		// Verify BLS proof of possession
		Expect(output).To(ContainSubstring("proof of possession verified"))
		
		// Check that all required files exist
		stakingDir := filepath.Join(testDir, "staking")
		Expect(filepath.Join(stakingDir, "staker.crt")).To(BeAnExistingFile())
		Expect(filepath.Join(stakingDir, "staker.key")).To(BeAnExistingFile())
		Expect(filepath.Join(stakingDir, "signer.key")).To(BeAnExistingFile())
		
		// Verify BLS key format
		signerKey, err := os.ReadFile(filepath.Join(stakingDir, "signer.key"))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(signerKey)).To(Equal(32), "BLS secret key should be 32 bytes")
	})
})