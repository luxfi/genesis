package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewToolsCmd creates the tools command
func NewToolsCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Various tools and utilities",
		Long:  "Collection of utility tools for blockchain operations",
	}

	// Subcommands
	cmd.AddCommand(newReplaceImportsCmd(app))
	cmd.AddCommand(&cobra.Command{
		Use:   "verify-genesis",
		Short: "Verify genesis file format",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Genesis verification not yet implemented")
			return nil
		},
	})

	return cmd
}

// newReplaceImportsCmd creates the command for replacing import paths in Go files.
// This replaces the functionality of the `replace-imports.sh` script.
func newReplaceImportsCmd(app *application.Genesis) *cobra.Command {
	var (
		oldImports []string
		newImports []string
		path       string
	)

	cmd := &cobra.Command{
		Use:   "replace-imports",
		Short: "Replace import paths in Go files",
		Long:  "Recursively finds all .go files in a directory and replaces specified import paths.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(oldImports) != len(newImports) {
				return fmt.Errorf("the number of old and new imports must be the same")
			}

			replacements := make(map[string]string)
			for i, oldImport := range oldImports {
				replacements[oldImport] = newImports[i]
			}

			return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
					content, err := ioutil.ReadFile(filePath)
					if err != nil {
						return err
					}

					newContent := string(content)
					for old, new := range replacements {
						newContent = strings.ReplaceAll(newContent, old, new)
					}

					if err := ioutil.WriteFile(filePath, []byte(newContent), info.Mode()); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().StringSliceVar(&oldImports, "old", []string{"github.com/ethereum/go-ethereum", "github.com/miguelmota/go-ethereum-hdwallet"}, "List of import paths to replace")
	cmd.Flags().StringSliceVar(&newImports, "new", []string{"github.com/luxfi/geth", "github.com/luxfi/hdwallet"}, "List of new import paths")
	cmd.Flags().StringVar(&path, "path", ".", "The root directory to search for Go files")

	return cmd
}