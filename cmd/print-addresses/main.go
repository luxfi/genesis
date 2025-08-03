package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/secp256k1"
	bip32 "github.com/luxfi/go-bip32"
	bip39 "github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/utils/constants"
)

const (
	// Address separator
	addressSep = "-"

	// BIP44 path for Lux: m/44'/9000'/0'/0/{account}
	// 9000 is the registered coin type for Lux/Avalanche
	purposeIndex  = 44
	coinTypeIndex = 9000
	accountIndex  = 0
	changeIndex   = 0
)

// AddressSet contains all address formats for a single account
type AddressSet struct {
	AccountIndex int    `json:"accountIndex,omitempty"`
	AddressID    string `json:"addressId,omitempty"`
	EVMAddress   string `json:"evmAddress"`
	CChain       string `json:"cChain"`
	PChain       string `json:"pChain"`
	XChain       string `json:"xChain"`
	PrivateKey   string `json:"privateKey,omitempty"`
	// Testnet addresses
	CChainTest string `json:"cChainTest,omitempty"`
	PChainTest string `json:"pChainTest,omitempty"`
	XChainTest string `json:"xChainTest,omitempty"`
}

// Result contains all address sets for output
type Result struct {
	Addresses []AddressSet `json:"addresses"`
}

var (
	numAccounts int
	outputJSON  bool
	showTestnet bool
)

func main() {
	// Parse command line arguments
	flag.IntVar(&numAccounts, "n", 1, "Number of accounts to generate (default: 1)")
	flag.BoolVar(&outputJSON, "json", false, "Output in JSON format")
	flag.BoolVar(&showTestnet, "testnet", false, "Also show testnet addresses")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <input>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nInput can be:\n")
		fmt.Fprintf(os.Stderr, "  - Mnemonic phrase (12-24 words)\n")
		fmt.Fprintf(os.Stderr, "  - Private key (hex)\n")
		fmt.Fprintf(os.Stderr, "  - EVM address (hex) - will show corresponding P/X addresses\n")
		fmt.Fprintf(os.Stderr, "  - Comma-separated list of any of the above\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s \"word1 word2 ... word12\"             # Mnemonic\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 0x1234...                             # Private key\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 0x742d35Cc6634C0532925a3b844Bc9e7595f # EVM address\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s \"0x123...,0x456...,0x789...\"         # Multiple inputs\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -n=5 \"mnemonic\"                      # Generate 5 accounts\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --json \"mnemonic\" > addresses.json   # Output as JSON\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --testnet \"mnemonic\"                 # Include testnet addresses\n", os.Args[0])
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	input := flag.Arg(0)
	var allAddresses []AddressSet

	// Check if input contains commas (multiple inputs)
	if strings.Contains(input, ",") {
		allAddresses = handleMultipleInputs(input)
	} else {
		// Single input
		allAddresses = processInput(input, numAccounts)
	}

	// Output results
	if outputJSON {
		result := Result{Addresses: allAddresses}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
	}
}

func handleMultipleInputs(input string) []AddressSet {
	var allAddresses []AddressSet
	inputs := strings.Split(input, ",")

	for i, inp := range inputs {
		inp = strings.TrimSpace(inp)
		if inp == "" {
			continue
		}

		if !outputJSON {
			if i > 0 {
				fmt.Println() // Empty line between inputs
			}
			fmt.Printf("=== Input #%d: %s ===\n", i+1, truncateString(inp, 20))
		}

		addresses := processInput(inp, 1)
		allAddresses = append(allAddresses, addresses...)
	}

	return allAddresses
}

func processInput(input string, accountCount int) []AddressSet {
	input = strings.TrimSpace(input)

	// Determine input type and process accordingly
	if isEVMAddress(input) {
		return handleEVMAddress(input)
	} else if strings.HasPrefix(input, "0x") || isHexString(input) {
		if len(strings.TrimPrefix(input, "0x")) == 40 {
			// 40 hex chars = 20 bytes = EVM address
			return handleEVMAddress(input)
		} else {
			// Assume private key
			return handlePrivateKey(input)
		}
	} else {
		// Assume mnemonic
		return handleMnemonic(input, accountCount)
	}
}

func isEVMAddress(s string) bool {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func isHexString(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func handleEVMAddress(evmAddr string) []AddressSet {
	// Remove 0x prefix if present
	evмAddr = strings.TrimPrefix(evmAddr, "0x")

	// Decode hex string
	addressBytes, err := hex.DecodeString(evmAddr)
	if err != nil {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Error decoding EVM address: %v\n", err)
		}
		return nil
	}

	if len(addressBytes) != 20 {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Invalid EVM address length: expected 20 bytes, got %d\n", len(addressBytes))
		}
		return nil
	}

	// For EVM addresses only, we cannot derive P/X addresses
	// because they use different hashing algorithms from the same public key
	if !outputJSON {
		fmt.Printf("EVM Address: 0x%s\n", evmAddr)
		fmt.Println("Note: P-Chain and X-Chain addresses cannot be derived from an EVM address alone.")
		fmt.Println("They require the original public key, which cannot be recovered from the EVM address.")
	}

	// Return minimal info for JSON output
	if outputJSON {
		addrSet := &AddressSet{
			EVMAddress: fmt.Sprintf("0x%s", evmAddr),
			CChain:     fmt.Sprintf("0x%s", evmAddr), // C-Chain uses EVM addresses
		}
		return []AddressSet{*addrSet}
	}

	return nil
}

func handlePrivateKey(hexKey string) []AddressSet {
	// Remove 0x prefix if present
	hexKey = strings.TrimPrefix(hexKey, "0x")

	// Decode hex string
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Error decoding hex private key: %v\n", err)
		}
		return nil
	}

	// Create private key
	privKey, err := secp256k1.ToPrivateKey(keyBytes)
	if err != nil {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Error creating private key: %v\n", err)
		}
		return nil
	}

	// Generate and print addresses
	addrSet := generateAddresses(privKey, 0)
	if !outputJSON && addrSet != nil {
		printAddressSet(*addrSet)
	}

	if addrSet != nil {
		return []AddressSet{*addrSet}
	}
	return nil
}

func handleMnemonic(mnemonic string, accountCount int) []AddressSet {
	// Validate mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Invalid mnemonic\n")
		}
		return nil
	}

	// Generate seed from mnemonic (no passphrase)
	seed := bip39.NewSeed(mnemonic, "")

	// Create master key
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Error creating master key: %v\n", err)
		}
		return nil
	}

	var allAddresses []AddressSet

	// Derive keys for each account
	for i := 0; i < accountCount; i++ {
		// Derive path: m/44'/9000'/0'/0/{i}
		key, err := deriveKey(masterKey, i)
		if err != nil {
			if !outputJSON {
				fmt.Fprintf(os.Stderr, "Error deriving key for account %d: %v\n", i, err)
			}
			continue
		}

		// Convert BIP32 key to secp256k1 private key
		privKey, err := secp256k1.ToPrivateKey(key.Key)
		if err != nil {
			if !outputJSON {
				fmt.Fprintf(os.Stderr, "Error converting key for account %d: %v\n", i, err)
			}
			continue
		}

		addrSet := generateAddresses(privKey, i)
		if addrSet != nil {
			allAddresses = append(allAddresses, *addrSet)
			if !outputJSON {
				printAddressSet(*addrSet)
				if i < accountCount-1 {
					fmt.Println() // Empty line between accounts
				}
			}
		}
	}

	return allAddresses
}

func deriveKey(masterKey *bip32.Key, accountIdx int) (*bip32.Key, error) {
	// m/44'
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + purposeIndex)
	if err != nil {
		return nil, err
	}

	// m/44'/9000'
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + coinTypeIndex)
	if err != nil {
		return nil, err
	}

	// m/44'/9000'/0'
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + accountIndex)
	if err != nil {
		return nil, err
	}

	// m/44'/9000'/0'/0
	change, err := account.NewChildKey(changeIndex)
	if err != nil {
		return nil, err
	}

	// m/44'/9000'/0'/0/{accountIdx}
	return change.NewChildKey(uint32(accountIdx))
}

// generateEVMAddress generates an Ethereum-style address from a public key
func generateEVMAddress(pubKey *ecdsa.PublicKey) []byte {
	return crypto.PubkeyToAddress(*pubKey).Bytes()
}

func generateAddresses(privKey *secp256k1.PrivateKey, accountIdx int) *AddressSet {
	// Get public key
	pubKey := privKey.PublicKey()

	// Get EVM address (Keccak256 based)
	evмAddressBytes := generateEVMAddress(privKey.ToECDSA().Public().(*ecdsa.PublicKey))

	// Get Lux native address bytes (for P/X chains)
	luxAddressBytes := pubKey.Address().Bytes()

	return generateAddressSet(evmAddressBytes, luxAddressBytes, accountIdx, true, fmt.Sprintf("0x%x", privKey.Bytes()))
}

func generateAddressSet(evmAddressBytes, luxAddressBytes []byte, accountIdx int, showAddressID bool, privKeyHex string) *AddressSet {
	// Generate mainnet addresses
	mainnetHRP := constants.GetHRP(constants.MainnetID)
	bech32AddrMainnet, err := formatBech32(mainnetHRP, luxAddressBytes)
	if err != nil {
		if !outputJSON {
			fmt.Fprintf(os.Stderr, "Error formatting mainnet bech32 address: %v\n", err)
		}
		return nil
	}

	addrSet := &AddressSet{
		EVMAddress: fmt.Sprintf("0x%x", evmAddressBytes),
		CChain:     fmt.Sprintf("C-%s", bech32AddrMainnet),
		PChain:     fmt.Sprintf("P-%s", bech32AddrMainnet),
		XChain:     fmt.Sprintf("X-%s", bech32AddrMainnet),
	}

	if accountIdx >= 0 {
		addrSet.AccountIndex = accountIdx
	}

	if showAddressID {
		addrID, _ := ids.ToShortID(luxAddressBytes)
		addrSet.AddressID = addrID.String()
	}

	if privKeyHex != "" && privKeyHex != "unknown" {
		addrSet.PrivateKey = privKeyHex
	}

	// Generate testnet addresses if requested
	if showTestnet {
		testnetHRP := constants.GetHRP(constants.TestnetID)
		bech32AddrTestnet, err := formatBech32(testnetHRP, luxAddressBytes)
		if err == nil {
			addrSet.CChainTest = fmt.Sprintf("C-%s", bech32AddrTestnet)
			addrSet.PChainTest = fmt.Sprintf("P-%s", bech32AddrTestnet)
			addrSet.XChainTest = fmt.Sprintf("X-%s", bech32AddrTestnet)
		}
	}

	return addrSet
}

func printAddressSet(addrSet AddressSet) {
	if addrSet.AccountIndex >= 0 {
		fmt.Printf("Account #%d:\n", addrSet.AccountIndex)
	}

	if addrSet.AddressID != "" {
		fmt.Printf("  Address ID: %s\n", addrSet.AddressID)
	}

	fmt.Printf("  EVM Address: %s\n", addrSet.EVMAddress)
	fmt.Printf("  C-Chain: %s\n", addrSet.CChain)
	fmt.Printf("  P-Chain: %s\n", addrSet.PChain)
	fmt.Printf("  X-Chain: %s\n", addrSet.XChain)

	if showTestnet && addrSet.CChainTest != "" {
		fmt.Println("  Testnet:")
		fmt.Printf("    C-Chain: %s\n", addrSet.CChainTest)
		fmt.Printf("    P-Chain: %s\n", addrSet.PChainTest)
		fmt.Printf("    X-Chain: %s\n", addrSet.XChainTest)
	}

	if addrSet.PrivateKey != "" && addrSet.PrivateKey != "unknown" {
		fmt.Printf("  Private Key: %s\n", addrSet.PrivateKey)
	}
}

// formatBech32 formats an address as bech32
func formatBech32(hrp string, payload []byte) (string, error) {
	// Convert 8-bit to 5-bit for bech32 encoding
	fiveBits, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %w", err)
	}

	// Encode to bech32
	return bech32.Encode(hrp, fiveBits)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
