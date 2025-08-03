package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/luxfi/crypto/bls"
)

type ProofOfPossession struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

type Staker struct {
	NodeID        string             `json:"nodeID"`
	RewardAddress string             `json:"rewardAddress,omitempty"`
	DelegationFee uint32             `json:"delegationFee"`
	Signer        *ProofOfPossession `json:"signer,omitempty"`
}

type UnlockSchedule struct {
	Amount   int64 `json:"amount"`
	Locktime int64 `json:"locktime"`
}

type Allocation struct {
	ETHAddr        string           `json:"ethAddr"`
	AVAXAddr       string           `json:"luxAddr"`
	InitialAmount  int64            `json:"initialAmount"`
	UnlockSchedule []UnlockSchedule `json:"unlockSchedule"`
}

type Genesis struct {
	NetworkID                  int          `json:"networkID"`
	Allocations                []Allocation `json:"allocations"`
	StartTime                  int64        `json:"startTime"`
	InitialStakeDuration       int64        `json:"initialStakeDuration"`
	InitialStakers             []Staker     `json:"initialStakers"`
	InitialStakedFunds         []string     `json:"initialStakedFunds"`
	CChainGenesis              string       `json:"cChainGenesis"`
	InitialStakeDurationOffset int64        `json:"initialStakeDurationOffset,omitempty"`
	Message                    string       `json:"message"`
}

func main() {
	// Generate BLS key
	sk, err := bls.NewSecretKey()
	if err != nil {
		panic(err)
	}

	pk := sk.PublicKey()
	pkBytes := bls.PublicKeyToCompressedBytes(pk)
	
	// Sign proof of possession
	sig := sk.SignProofOfPossession(pkBytes)
	sigBytes := bls.SignatureToBytes(sig)

	// Create proof of possession
	pop := &ProofOfPossession{
		PublicKey:         fmt.Sprintf("0x%x", pkBytes),
		ProofOfPossession: fmt.Sprintf("0x%x", sigBytes),
	}

	// Node ID from ephemeral cert logs
	nodeID := "NodeID-111111111111111111116DBWJs"
	
	// Staking address
	stakingAddress := "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5"
	
	// Create staker
	staker := Staker{
		NodeID:        nodeID,
		RewardAddress: stakingAddress,
		DelegationFee: 20000, // 2%
		Signer:        pop,
	}

	// Genesis parameters
	startTime := int64(1667900400)
	oneWeekLater := startTime + 7*24*60*60
	
	// Create allocations
	allocations := []Allocation{
		{
			// Regular funding for X-Chain
			ETHAddr:       "0x0000000000000000000000000000000000000000",
			AVAXAddr:      "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			InitialAmount: 0,
			UnlockSchedule: []UnlockSchedule{
				{
					Amount:   100000000000000000, // 100M LUX unlocked
					Locktime: 0,
				},
			},
		},
		{
			// Staking allocation - provides weight to validator
			ETHAddr:       "0x0000000000000000000000000000000000000000",
			AVAXAddr:      stakingAddress,
			InitialAmount: 0,
			UnlockSchedule: []UnlockSchedule{
				{
					Amount:   2000000000000, // 2000 LUX (minimum stake) 
					Locktime: oneWeekLater,
				},
			},
		},
		{
			// C-Chain funding
			ETHAddr:       "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
			AVAXAddr:      "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			InitialAmount: 100000000000000000, // 100M LUX on C-Chain
			UnlockSchedule: []UnlockSchedule{},
		},
	}

	// Create genesis
	genesis := Genesis{
		NetworkID:                  96369,
		Allocations:                allocations,
		StartTime:                  startTime,
		InitialStakeDuration:       31536000, // 1 year
		InitialStakers:             []Staker{staker},
		InitialStakedFunds:         []string{stakingAddress},
		InitialStakeDurationOffset: 5400, // 90 minutes
		CChainGenesis:              `{"config":{"chainId":96369,"homesteadBlock":0,"eip150Block":0,"eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0,"cancunBlock":0,"shanghaiTime":0,"terminalTotalDifficulty":0},"difficulty":"0x0","gasLimit":"0x7A1200","alloc":{"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC":{"balance":"0x3635c9adc5dea00000"}}}`,
		Message:                    "Lux mainnet genesis with proper BLS validator and staking allocation",
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		panic(err)
	}

	// Write to file
	err = os.WriteFile("/home/z/.luxd/configs/genesis-bls.json", data, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("Genesis file created with BLS validator and proper staking allocation")
	fmt.Printf("Node ID: %s\n", nodeID)
	fmt.Printf("Public Key: %s\n", pop.PublicKey)
	fmt.Printf("Proof of Possession: %s\n", pop.ProofOfPossession)
	fmt.Printf("Staking Address: %s\n", stakingAddress)
}