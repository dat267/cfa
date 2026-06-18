package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// ImportCommand represents the import vault entries command.
type ImportCommand struct {
	fs        *flag.FlagSet
	vaultPath string
	inOpt     string
}

func NewImportCommand(vaultPath string) *ImportCommand {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	c := &ImportCommand{fs: fs, vaultPath: vaultPath}
	fs.StringVar(&c.inOpt, "in", "", "Input JSON file path")
	return c
}

func (c *ImportCommand) Name() string        { return "import" }
func (c *ImportCommand) Description() string { return "Import entries from a plain JSON file" }
func (c *ImportCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *ImportCommand) Run(args []string) error {
	password, err := getVaultPassword(c.vaultPath)
	if err != nil {
		return err
	}

	var inputData []byte
	if c.inOpt != "" {
		data, err := os.ReadFile(c.inOpt)
		if err != nil {
			return fmt.Errorf("failed to read import file: %w", err)
		}
		inputData = data
	} else {
		fmt.Println("Reading JSON from standard input... (Press Ctrl+D when finished)")
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		inputData = data
	}

	var importedEntries []VaultEntry
	if err := json.Unmarshal(inputData, &importedEntries); err != nil {
		return fmt.Errorf("invalid import JSON: %w", err)
	}

	for i, entry := range importedEntries {
		if entry.Name == "" {
			return fmt.Errorf("entry #%d is missing account name", i+1)
		}
		entry.Secret = CleanSecret(entry.Secret)
		if err := ValidateBase32(entry.Secret); err != nil {
			return fmt.Errorf("entry '%s' has invalid secret: %w", entry.Name, err)
		}
		if entry.Algorithm == "" {
			entry.Algorithm = "SHA1"
		}
		if _, err := ParseAlgorithm(entry.Algorithm); err != nil {
			return fmt.Errorf("entry '%s' has invalid algorithm: %w", entry.Name, err)
		}
		if entry.Digits == 0 {
			entry.Digits = 6
		}
		if _, err := ParseDigits(entry.Digits); err != nil {
			return fmt.Errorf("entry '%s' has invalid digits: %w", entry.Name, err)
		}
		if entry.Period == 0 {
			entry.Period = 30
		}
		importedEntries[i] = entry
	}

	existingEntries, err := LoadVault(c.vaultPath, password)
	if err != nil {
		return err
	}

	mergedCount := 0
	addedCount := 0

	for _, imported := range importedEntries {
		found := false
		for i, existing := range existingEntries {
			if strings.EqualFold(existing.Name, imported.Name) {
				existingEntries[i] = imported
				found = true
				mergedCount++
				break
			}
		}
		if !found {
			existingEntries = append(existingEntries, imported)
			addedCount++
		}
	}

	if err := SaveVault(c.vaultPath, existingEntries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccess: Imported %d entries (%d added, %d updated)\033[0m\n", len(importedEntries), addedCount, mergedCount)
	return nil
}
