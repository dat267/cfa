package cmd

import (
	"flag"
	"fmt"
	"strings"
	"syscall"

	"golang.org/x/term"
)


// AddCommand represents the command to add a new TOTP credential.
type AddCommand struct {
	fs        *flag.FlagSet
	secretOpt string
	qrOpt     string
	issuerOpt string
	algoOpt   string
	digitsOpt int
	periodOpt uint
}

func NewAddCommand() *AddCommand {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	c := &AddCommand{fs: fs}
	fs.StringVar(&c.secretOpt, "secret", "", "MFA secret key (Base32)")
	fs.StringVar(&c.qrOpt, "qr", "", "Path to a QR code image file")
	fs.StringVar(&c.issuerOpt, "issuer", "", "MFA issuer")
	fs.StringVar(&c.algoOpt, "algo", "SHA1", "Hashing algorithm (SHA1, SHA256, SHA512)")
	fs.IntVar(&c.digitsOpt, "digits", 6, "Number of digits (6 or 8)")
	fs.UintVar(&c.periodOpt, "period", 30, "Time period in seconds")
	return c
}

func (c *AddCommand) Name() string        { return "add" }
func (c *AddCommand) Description() string { return "Add a new MFA token" }
func (c *AddCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *AddCommand) Run(positional []string) error {
	var entry *VaultEntry
	var name string

	if len(positional) > 0 {
		name = positional[0]
	}

	// Case 1: Load from QR code
	if c.qrOpt != "" {
		fmt.Printf("Decoding QR code from %s...\n", c.qrOpt)
		decoded, err := DecodeQRCode(c.qrOpt)
		if err != nil {
			return err
		}

		if strings.HasPrefix(decoded, "otpauth://") {
			parsed, err := ParseOTPAuthURL(decoded)
			if err != nil {
				return fmt.Errorf("failed to parse OTP URI from QR code: %w", err)
			}
			entry = parsed
			if name != "" {
				entry.Name = name // Override name if user explicitly provided one
			}
		} else {
			// Assume it's a raw secret inside the QR
			secret := CleanSecret(decoded)
			if err := ValidateBase32(secret); err != nil {
				return fmt.Errorf("QR code content is not a valid OTP URI or Base32 secret: %w", err)
			}
			entry = &VaultEntry{
				Secret:    secret,
				Algorithm: strings.ToUpper(c.algoOpt),
				Digits:    c.digitsOpt,
				Period:    c.periodOpt,
				Issuer:    c.issuerOpt,
			}
		}
	} else {
		// Case 2: Load from manual secret or prompt
		secret := c.secretOpt
		if secret == "" {
			fmt.Print("Enter secret key (Base32): ")
			byteSecret, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read secret key: %w", err)
			}
			fmt.Println()
			secret = string(byteSecret)
		}

		secret = CleanSecret(secret)
		if err := ValidateBase32(secret); err != nil {
			return err
		}

		entry = &VaultEntry{
			Secret:    secret,
			Algorithm: strings.ToUpper(c.algoOpt),
			Digits:    c.digitsOpt,
			Period:    c.periodOpt,
			Issuer:    c.issuerOpt,
		}
	}

	// Validate algorithms and digits parameters
	if _, err := ParseAlgorithm(entry.Algorithm); err != nil {
		return err
	}
	if _, err := ParseDigits(entry.Digits); err != nil {
		return err
	}

	// Ask for account name if still empty
	if name == "" && entry.Name == "" {
		fmt.Print("Enter account name (e.g. GitHub:john): ")
		var inputName string
		fmt.Scanln(&inputName)
		inputName = strings.TrimSpace(inputName)
		if inputName == "" {
			return fmt.Errorf("account name cannot be empty")
		}
		entry.Name = inputName
	} else if name != "" {
		entry.Name = name
	}

	// Ask for master password to unlock and write to vault
	password, err := getVaultPassword()
	if err != nil {
		return err
	}

	entries, err := LoadVault(VaultPath, password)
	if err != nil {
		return err
	}

	// Check if name already exists
	for i, existing := range entries {
		if strings.EqualFold(existing.Name, entry.Name) {
			fmt.Printf("An account named '%s' already exists. Overwrite? [y/N]: ", entry.Name)
			var resp string
			fmt.Scanln(&resp)
			resp = strings.ToLower(strings.TrimSpace(resp))
			if resp != "y" && resp != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
			// Overwrite the existing entry
			entries[i] = *entry
			if err := SaveVault(VaultPath, entries, password); err != nil {
				return err
			}
			fmt.Printf("\033[32mSuccessfully updated account '%s'\033[0m\n", entry.Name)
			return nil
		}
	}

	// Add new entry
	entries = append(entries, *entry)
	if err := SaveVault(VaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully added account '%s'\033[0m\n", entry.Name)
	return nil
}
