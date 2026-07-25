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

	for i := range importedEntries {
		entry := &importedEntries[i]
		if entry.Name == "" {
			return fmt.Errorf("entry #%d is missing account name", i+1)
		}
		entry.Secret = cleanSecret(entry.Secret)
		if err := validateBase32(entry.Secret); err != nil {
			return fmt.Errorf("entry '%s' has invalid secret: %w", entry.Name, err)
		}
		if entry.Algorithm == "" {
			entry.Algorithm = "SHA1"
		}
		if _, err := parseAlgorithm(entry.Algorithm); err != nil {
			return fmt.Errorf("entry '%s' has invalid algorithm: %w", entry.Name, err)
		}
		if entry.Digits == 0 {
			entry.Digits = 6
		}
		if _, err := parseDigits(entry.Digits); err != nil {
			return fmt.Errorf("entry '%s' has invalid digits: %w", entry.Name, err)
		}
		if entry.Period == 0 {
			entry.Period = 30
		}
	}

	existingEntries, password, err := vaultPath.Open()
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

	if err := vaultPath.Save(existingEntries, password); err != nil {
		return err
	}

	fmt.Printf(colorGreen+"Success: Imported %d entries (%d added, %d updated)"+colorReset+"\n", len(importedEntries), addedCount, mergedCount)
	return nil
}
