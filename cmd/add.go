package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type AddCmd struct {
	Name    string `arg:"" optional:"" help:"Account name"`
	Secret  string `help:"MFA secret key (Base32)"`
	QR      string `help:"Path to a QR code image file"`
	Issuer  string `help:"MFA issuer (e.g. GitHub, Google)"`
	Algo    string `help:"Hashing algorithm (SHA1, SHA256, SHA512)" default:"SHA1"`
	Digits  int    `help:"Number of code digits (6 or 8)" default:"6"`
	Period  uint   `help:"Time step period (seconds)" default:"30"`
}

func (c *AddCmd) Run(vaultPath VaultPath) error {
	var entry *VaultEntry
	var name string

	if c.Name != "" {
		name = c.Name
	}

	if c.QR != "" {
		fmt.Printf("Decoding QR code from %s...\n", c.QR)
		decoded, err := decodeQRCode(c.QR)
		if err != nil {
			return fmt.Errorf("failed to decode QR code: %w", err)
		}

		if strings.HasPrefix(decoded, "otpauth://") {
			parsed, err := parseOTPAuthURL(decoded)
			if err != nil {
				return fmt.Errorf("failed to parse OTP URI from QR code: %w", err)
			}
			entry = parsed
			if name != "" {
				entry.Name = name
			}
		} else {
			secret := cleanSecret(decoded)
			if err := validateBase32(secret); err != nil {
				return fmt.Errorf("QR code content is not a valid OTP URI or Base32 secret: %w", err)
			}
			entry = &VaultEntry{
				Secret:    secret,
				Algorithm: strings.ToUpper(c.Algo),
				Digits:    c.Digits,
				Period:    c.Period,
				Issuer:    c.Issuer,
			}
		}
	} else {
		secret := c.Secret
		if secret == "" {
			fmt.Print("Enter secret key (Base32): ")
			byteSecret, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return fmt.Errorf("failed to read secret key: %w", err)
			}
			fmt.Println()
			secret = string(byteSecret)
		}

		secret = cleanSecret(secret)
		if err := validateBase32(secret); err != nil {
			return err
		}

		entry = &VaultEntry{
			Secret:    secret,
			Algorithm: strings.ToUpper(c.Algo),
			Digits:    c.Digits,
			Period:    c.Period,
			Issuer:    c.Issuer,
		}
	}

	if _, err := parseAlgorithm(entry.Algorithm); err != nil {
		return err
	}
	if _, err := parseDigits(entry.Digits); err != nil {
		return err
	}

	if name == "" && entry.Name == "" {
		fmt.Print("Enter account name (e.g. GitHub:john): ")
		var inputName string
		if _, err := fmt.Scanln(&inputName); err != nil {
			return fmt.Errorf("failed to read account name: %w", err)
		}
		inputName = strings.TrimSpace(inputName)
		if inputName == "" {
			return fmt.Errorf("account name cannot be empty")
		}
		entry.Name = inputName
	} else if name != "" {
		entry.Name = name
	}

	password, err := getVaultPassword(string(vaultPath))
	if err != nil {
		return err
	}

	entries, err := LoadVault(string(vaultPath), password)
	if err != nil {
		return err
	}

	for i, existing := range entries {
		if strings.EqualFold(existing.Name, entry.Name) {
			ok, err := confirmAction("An account named '%s' already exists. Overwrite?", entry.Name)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
			entries[i] = *entry
			if err := vaultPath.Save(entries, password); err != nil {
				return err
			}
			fmt.Printf(colorGreen+"Successfully updated account '%s'"+colorReset+"\n", entry.Name)
			return nil
		}
	}

	entries = append(entries, *entry)
	if err := vaultPath.Save(entries, password); err != nil {
		return err
	}

	fmt.Printf(colorGreen+"Successfully added account '%s'"+colorReset+"\n", entry.Name)
	return nil
}
