package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type ImportCmd struct {
	In string `help:"Input JSON file path" placeholder:"PATH"`
}

func (c *ImportCmd) Run(vaultPath VaultPath) error {
	password, err := getVaultPassword(string(vaultPath))
	if err != nil {
		return err
	}

	var inputData []byte
	if c.In != "" {
		data, err := os.ReadFile(c.In)
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

	existingEntries, err := LoadVault(string(vaultPath), password)
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

	if err := SaveVault(string(vaultPath), existingEntries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccess: Imported %d entries (%d added, %d updated)\033[0m\n", len(importedEntries), addedCount, mergedCount)
	return nil
}
