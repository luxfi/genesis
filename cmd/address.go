package cmd

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/utils/constants"
	"github.com/spf13/cobra"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
)

const (
	// BIP44 path for Lux: m/44'/9000'/0'/0/{account}
	// 9000 is the registered coin type for Lux/Avalanche
	purposeIndex  = 44
	coinTypeIndex = 9000
	accountIndex  = 0
	changeIndex   = 0
)

var (
	// Address command flags
	addrNumAccounts int
	addrShowPrivKey bool
	addrOutputJSON  bool
	addrShowTestnet bool
)

// NewAddressCmd creates the address command
func NewAddressCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Address utilities",
		Long:  "Generate and manage addresses from mnemonics or private keys",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	// Add subcommands
	cmd.AddCommand(newAddressGenerateCmd(app))
	cmd.AddCommand(newAddressConvertCmd(app))

	return cmd
}

func newAddressGenerateCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [mnemonic]",
		Short: "Generate addresses from mnemonic",
		Long: `Generate addresses from a BIP39 mnemonic phrase.

Examples:
  genesis address generate "word1 word2 ... word12"
  genesis address generate -n=5 "mnemonic"
  genesis address generate --priv "mnemonic"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mnemonic := strings.TrimSpace(args[0])
			return generateAddresses(app, mnemonic)
		},
	}

	cmd.Flags().IntVarP(&addrNumAccounts, "num", "n", 1, "Number of accounts to generate")
	cmd.Flags().BoolVar(&addrShowPrivKey, "priv", false, "Show private keys")
	cmd.Flags().BoolVar(&addrOutputJSON, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&addrShowTestnet, "testnet", false, "Also show testnet addresses")

	return cmd
}

func newAddressConvertCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [address/key]",
		Short: "Convert between address formats",
		Long: `Convert between different address formats.

Input can be:
  - EVM address (0x...)
  - Private key (hex)
  - P/X/C-Chain address`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			return convertAddress(app, input)
		},
	}

	cmd.Flags().BoolVar(&addrOutputJSON, "json", false, "Output in JSON format")

	return cmd
}

// AddressSet contains all address formats
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

func generateAddresses(app *application.Genesis, mnemonic string) error {
	// Validate mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("invalid mnemonic")
	}

	// Generate seed from mnemonic (no passphrase)
	seed := bip39.NewSeed(mnemonic, "")

	// Create master key
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return fmt.Errorf("error creating master key: %w", err)
	}

	var allAddresses []AddressSet

	// Derive keys for each account
	for i := 0; i < addrNumAccounts; i++ {
		// Derive path: m/44'/9000'/0'/0/{i}
		key, err := deriveKey(masterKey, i)
		if err != nil {
			app.Log.Error("Error deriving key", "account", i, "error", err)
			continue
		}

		// Convert BIP32 key to secp256k1 private key
		privKey, err := secp256k1.ToPrivateKey(key.Key)
		if err != nil {
			app.Log.Error("Error converting key", "account", i, "error", err)
			continue
		}

		addrSet := generateAddressSet(privKey, i)
		if addrSet != nil {
			allAddresses = append(allAddresses, *addrSet)
			if !addrOutputJSON {
				printAddressSet(*addrSet)
				if i < addrNumAccounts-1 {
					fmt.Println() // Empty line between accounts
				}
			}
		}
	}

	if addrOutputJSON {
		// TODO: Implement JSON output
		fmt.Println("JSON output not yet implemented")
	}

	return nil
}

func convertAddress(app *application.Genesis, input string) error {
	// TODO: Implement address conversion logic
	app.Log.Info("Address conversion not yet implemented", "input", input)
	return nil
}

func deriveKey(masterKey *bip32.Key, idx int) (*bip32.Key, error) {
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

	// m/44'/9000'/0'/0/{idx}
	return change.NewChildKey(uint32(idx))
}

func generateAddressSet(privKey *secp256k1.PrivateKey, idx int) *AddressSet {
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
		return nil
	}

	addrSet := &AddressSet{
		AccountIndex: idx,
		EVMAddress:   fmt.Sprintf("0x%x", evmAddressBytes),
		CChain:       fmt.Sprintf("C-%s", bech32AddrMainnet),
		PChain:       fmt.Sprintf("P-%s", bech32AddrMainnet),
		XChain:       fmt.Sprintf("X-%s", bech32AddrMainnet),
	}

	addrID, _ := ids.ToShortID(luxAddressBytes)
	addrSet.AddressID = addrID.String()

	if addrShowPrivKey {
		addrSet.PrivateKey = fmt.Sprintf("0x%x", privKey.Bytes())
	}

	// Generate testnet addresses if requested
	if addrShowTestnet {
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

func generateEVMAddress(pubKey *ecdsa.PublicKey) []byte {
	return crypto.PubkeyToAddress(*pubKey).Bytes()
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

	if addrShowTestnet && addrSet.CChainTest != "" {
		fmt.Println("  Testnet:")
		fmt.Printf("    C-Chain: %s\n", addrSet.CChainTest)
		fmt.Printf("    P-Chain: %s\n", addrSet.PChainTest)
		fmt.Printf("    X-Chain: %s\n", addrSet.XChainTest)
	}

	if addrSet.PrivateKey != "" {
		fmt.Printf("  Private Key: %s\n", addrSet.PrivateKey)
	}
}

func formatBech32(hrp string, payload []byte) (string, error) {
	// Convert 8-bit to 5-bit for bech32 encoding
	fiveBits, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %w", err)
	}

	// Encode to bech32
	return bech32.Encode(hrp, fiveBits)
}