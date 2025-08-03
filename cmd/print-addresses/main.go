package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/utils/constants"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
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
}

var (
	numAccounts int
	showPrivKey bool
)

func main() {
	// Parse command line arguments
	flag.IntVar(&numAccounts, "n", 1, "Number of accounts to generate (default: 1)")
	flag.BoolVar(&showPrivKey, "priv", false, "Show private keys")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <mnemonic>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s \"word1 word2 ... word12\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -n=5 \"mnemonic\"\n", os.Args[0])
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	mnemonic := strings.TrimSpace(flag.Arg(0))
	
	// Validate mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		fmt.Fprintf(os.Stderr, "Invalid mnemonic\n")
		os.Exit(1)
	}

	// Generate seed from mnemonic (no passphrase)
	seed := bip39.NewSeed(mnemonic, "")

	// Create master key
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating master key: %v\n", err)
		os.Exit(1)
	}

	// Derive keys for each account
	for i := 0; i < numAccounts; i++ {
		// Derive path: m/44'/9000'/0'/0/{i}
		key, err := deriveKey(masterKey, i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deriving key for account %d: %v\n", i, err)
			continue
		}

		// Convert BIP32 key to secp256k1 private key
		privKey, err := secp256k1.ToPrivateKey(key.Key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting key for account %d: %v\n", i, err)
			continue
		}

		addrSet := generateAddresses(privKey, i)
		if addrSet != nil {
			printAddressSet(*addrSet)
			if i < numAccounts-1 {
				fmt.Println() // Empty line between accounts
			}
		}
	}
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
	evmAddressBytes := generateEVMAddress(privKey.ToECDSA().Public().(*ecdsa.PublicKey))

	// Get Lux native address bytes (for P/X chains)
	luxAddressBytes := pubKey.Address().Bytes()

	// Generate mainnet addresses
	mainnetHRP := constants.GetHRP(constants.MainnetID)
	bech32AddrMainnet, err := formatBech32(mainnetHRP, luxAddressBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting mainnet bech32 address: %v\n", err)
		return nil
	}

	addrSet := &AddressSet{
		AccountIndex: accountIdx,
		EVMAddress:   fmt.Sprintf("0x%x", evmAddressBytes),
		CChain:       fmt.Sprintf("C-%s", bech32AddrMainnet),
		PChain:       fmt.Sprintf("P-%s", bech32AddrMainnet),
		XChain:       fmt.Sprintf("X-%s", bech32AddrMainnet),
	}

	addrID, _ := ids.ToShortID(luxAddressBytes)
	addrSet.AddressID = addrID.String()

	if showPrivKey {
		addrSet.PrivateKey = fmt.Sprintf("0x%x", privKey.Bytes())
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

	if addrSet.PrivateKey != "" {
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