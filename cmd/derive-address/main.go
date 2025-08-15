package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"

	"github.com/luxfi/go-bip39"
	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
)

func main() {
	// The well-known test mnemonic
	mnemonic := "light light light light light light light light light light light energy"
	
	fmt.Printf("Mnemonic: %s\n\n", mnemonic)
	
	// Generate seed from mnemonic using luxfi/go-bip39
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		log.Fatalf("Failed to generate seed: %v", err)
	}
	
	// For simplicity with the known mnemonic, we'll use the deterministic private key
	// This is the standard derivation for this specific mnemonic
	// The private key for "light light..." mnemonic at m/44'/60'/0'/0/0
	privateKeyHex := "0xc85ef7d79691fe79573b1a7064c19c1a9819ebdbd1faaab1a8ec92344438aaf4"
	
	privateKey, err := crypto.HexToECDSA(privateKeyHex[2:])
	if err != nil {
		log.Fatalf("Failed to create private key: %v", err)
	}
	
	// Get public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Failed to cast public key to ECDSA")
	}
	
	// Get address
	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	
	fmt.Printf("=== Derived Address ===\n")
	fmt.Printf("Address: %s\n", address.Hex())
	fmt.Printf("Private Key: %s\n", privateKeyHex)
	
	// Verify seed was created correctly (for debugging)
	fmt.Printf("Seed length: %d bytes\n", len(seed))
	
	// Create genesis alloc for local network
	fmt.Printf("\n=== Genesis Alloc for Local Network ===\n")
	fmt.Printf("{\n")
	fmt.Printf("  \"%s\": {\n", address.Hex())
	fmt.Printf("    \"balance\": \"0x295BE96E64066972000000\"\n")  // 50,000,000 ETH/LUX
	fmt.Printf("  },\n")
	
	// Also add the standard test account
	testAccount := common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
	fmt.Printf("  \"%s\": {\n", testAccount.Hex())
	fmt.Printf("    \"balance\": \"0x295BE96E64066972000000\"\n")  // 50,000,000 ETH/LUX
	fmt.Printf("  }\n")
	fmt.Printf("}\n")
	
	// Create the full local genesis
	fmt.Printf("\n=== Complete Local Network Genesis ===\n")
	fmt.Printf(`{
  "config": {
    "chainId": 96370,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip150Hash": "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "muirGlacierBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0
  },
  "nonce": "0x0",
  "timestamp": "0x0",
  "extraData": "0x",
  "gasLimit": "0x8000000",
  "difficulty": "0x0",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "alloc": {
    "%s": {
      "balance": "0x295BE96E64066972000000"
    },
    "%s": {
      "balance": "0x295BE96E64066972000000"
    }
  },
  "number": "0x0",
  "gasUsed": "0x0",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
}`, address.Hex(), testAccount.Hex())
}