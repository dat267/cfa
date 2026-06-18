package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

var version = "cfa/development"

func main() {
	if err := runCLI(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`cfa - Cryptographically Secure CLI MFA Code Generator

Usage:
  cfa <command> [arguments]

Commands:
  init                           Initialize the secure vault and set a master password
  add [name]                     Add a new MFA token (from prompt, QR code image, or raw secret)
  list                           Display a live-updating dashboard of all TOTP codes
  show <name>                    Show the current 6/8-digit code for a specific account
  remove <name>                  Delete an account from the vault
  rename <old_name> <new_name>   Rename an account
  passwd                         Change your master password
  export                         Export all entries as plain JSON (to stdout or file)
  import                         Import entries from a plain JSON file
  version                        Print version information

Options for 'add':
  --secret <key>                 MFA secret key (Base32)
  --qr <image_path>              Path to a QR code image file
  --issuer <issuer>              MFA issuer (e.g. GitHub, Google)
  --algo <SHA1|SHA256|SHA512>    Hashing algorithm (default: SHA1)
  --digits <6|8>                 Number of code digits (default: 6)
  --period <seconds>             Time step period (default: 30)

Options for 'show':
  -c, --copy                     Copy the generated code to the clipboard
  --secret                       Show the raw Base32 secret key instead of the code

Options for 'list':
  --static                       Output static list once and exit (for scripting/pipes)

Options for 'export':
  --out <file_path>              Output file path (default: stdout)

Options for 'import':
  --in <file_path>               Input file path (default: stdin)

Environment Variables:
  CFA_VAULT_PATH                 Override default vault file location (~/.config/cfa/vault.enc)
  CFA_PASSWORD                   Set master password non-interactively (useful for scripts)

Note: Running 'cfa' with no arguments will default to 'cfa list' (interactive mode).
`)
}

func runCLI() error {
	vaultPath, err := DefaultVaultPath()
	if err != nil {
		return fmt.Errorf("failed to determine vault path: %w", err)
	}

	if len(os.Args) < 2 {
		// Default behavior: if vault exists, list. Otherwise, print usage.
		if _, err := os.Stat(vaultPath); err == nil {
			return handleList(vaultPath, []string{})
		}
		printUsage()
		return nil
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version", "-v", "--version":
		fmt.Printf("cfa version %s\n", version)
		return nil
	case "init":
		return handleInit(vaultPath)
	case "add":
		return handleAdd(vaultPath, args)
	case "list":
		return handleList(vaultPath, args)
	case "show", "get":
		return handleShow(vaultPath, args)
	case "remove", "delete", "rm":
		return handleRemove(vaultPath, args)
	case "rename", "mv":
		return handleRename(vaultPath, args)
	case "passwd", "password", "change-password":
		return handlePasswd(vaultPath)
	case "export":
		return handleExport(vaultPath, args)
	case "import":
		return handleImport(vaultPath, args)
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func getVaultPassword(vaultPath string) (string, error) {
	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		return "", fmt.Errorf("vault not initialized. Please run: cfa init")
	}
	return GetMasterPassword("Enter master password: ", false)
}

func handleInit(vaultPath string) error {
	if _, err := os.Stat(vaultPath); err == nil {
		// Ask if they want to overwrite it
		fmt.Printf("\033[33mWarning: Vault already exists at %s.\033[0m\n", vaultPath)
		fmt.Print("Do you want to re-initialize it? All current secrets will be lost! [y/N]: ")
		var resp string
		fmt.Scanln(&resp)
		resp = strings.ToLower(strings.TrimSpace(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	pwd, err := GetMasterPassword("Set a master password: ", true)
	if err != nil {
		return err
	}

	var emptyEntries []VaultEntry
	if err := SaveVault(vaultPath, emptyEntries, pwd); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf("\033[32mSuccess: Vault securely initialized at %s\033[0m\n", vaultPath)
	return nil
}

func handleAdd(vaultPath string, args []string) error {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	secretOpt := addCmd.String("secret", "", "MFA secret key (Base32)")
	qrOpt := addCmd.String("qr", "", "Path to a QR code image file")
	issuerOpt := addCmd.String("issuer", "", "MFA issuer")
	algoOpt := addCmd.String("algo", "SHA1", "Hashing algorithm (SHA1, SHA256, SHA512)")
	digitsOpt := addCmd.Int("digits", 6, "Number of digits (6 or 8)")
	periodOpt := addCmd.Uint("period", 30, "Time period in seconds")

	positional, err := parseFlagsAndPositional(addCmd, args)
	if err != nil {
		return err
	}

	var entry *VaultEntry
	var name string

	// Parse positional argument for account name
	if len(positional) > 0 {
		name = positional[0]
	}

	// Case 1: Load from QR code
	if *qrOpt != "" {
		fmt.Printf("Decoding QR code from %s...\n", *qrOpt)
		decoded, err := DecodeQRCode(*qrOpt)
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
				Algorithm: strings.ToUpper(*algoOpt),
				Digits:    *digitsOpt,
				Period:    *periodOpt,
				Issuer:    *issuerOpt,
			}
		}
	} else {
		// Case 2: Load from manual secret or prompt
		secret := *secretOpt
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
			Algorithm: strings.ToUpper(*algoOpt),
			Digits:    *digitsOpt,
			Period:    *periodOpt,
			Issuer:    *issuerOpt,
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
	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
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
			if err := SaveVault(vaultPath, entries, password); err != nil {
				return err
			}
			fmt.Printf("\033[32mSuccessfully updated account '%s'\033[0m\n", entry.Name)
			return nil
		}
	}

	// Add new entry
	entries = append(entries, *entry)
	if err := SaveVault(vaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully added account '%s'\033[0m\n", entry.Name)
	return nil
}

func handleList(vaultPath string, args []string) error {
	listCmd := flag.NewFlagSet("list", flag.ContinueOnError)
	staticOpt := listCmd.Bool("static", false, "Output static list once and exit")

	if _, err := parseFlagsAndPositional(listCmd, args); err != nil {
		return err
	}

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No accounts found. Add one with: cfa add <name>")
		return nil
	}

	// If --static is enabled or stdout is not a TTY, print static output
	if *staticOpt || !term.IsTerminal(int(os.Stdout.Fd())) {
		t := time.Now()
		fmt.Printf("%-30s %-10s %-5s %s\n", "Account", "Code", "Rem", "Algorithm")
		fmt.Println(strings.Repeat("-", 60))
		for _, entry := range entries {
			code, err := GenerateTOTP(entry, t)
			if err != nil {
				code = "ERROR"
			}
			period := entry.Period
			if period == 0 {
				period = 30
			}
			rem := int(period) - int(t.Unix()%int64(period))
			fmt.Printf("%-30s %-10s %2ds  %s (%d digits)\n", entry.Name, code, rem, entry.Algorithm, entry.Digits)
		}
		return nil
	}

	// Run interactive dashboard
	runLiveView(vaultPath, password)
	return nil
}

func runLiveView(vaultPath string, password string) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Hide cursor
	fmt.Print("\033[?25l")
	// Restore cursor on exit
	defer fmt.Print("\033[?25h\n")

	fmt.Print("\033[H\033[J") // Clear screen initially

	for {
		select {
		case <-sigChan:
			return
		case <-ticker.C:
			entries, err := LoadVault(vaultPath, password)
			if err != nil {
				fmt.Printf("\rError reloading vault: %v\n", err)
				return
			}

			if len(entries) == 0 {
				fmt.Print("\033[H\033[J")
				fmt.Println("No accounts found. Add one with: cfa add <name>")
				return
			}

			fmt.Print("\033[H") // Move cursor to top-left

			t := time.Now()
			fmt.Printf("\033[1;36m=== MFA Code Generator (cfa) ===\033[0m  Local Time: %s\n\n", t.Format("15:04:05"))
			fmt.Printf("%-30s %-10s %-30s\n", "\033[1mAccount\033[0m", "\033[1mCode\033[0m", "\033[1mTime Remaining\033[0m")
			fmt.Println(strings.Repeat("-", 75))

			for _, entry := range entries {
				code, err := GenerateTOTP(entry, t)
				if err != nil {
					code = "ERROR"
				} else if len(code) == 6 {
					code = code[:3] + " " + code[3:]
				} else if len(code) == 8 {
					code = code[:4] + " " + code[4:]
				}

				period := entry.Period
				if period == 0 {
					period = 30
				}
				rem := int(period) - int(t.Unix()%int64(period))

				timeColor := "\033[32m" // Green
				if rem <= 5 {
					timeColor = "\033[31m" // Red
				} else if rem <= 10 {
					timeColor = "\033[33m" // Yellow
				}

				barWidth := 20
				filled := (rem * barWidth) / int(period)
				if filled < 0 {
					filled = 0
				} else if filled > barWidth {
					filled = barWidth
				}
				empty := barWidth - filled
				bar := strings.Repeat("=", filled) + strings.Repeat(" ", empty)

				fmt.Printf("%-30s \033[1;32m%-10s\033[0m %s[%s] %2ds remaining\033[0m\n",
					entry.Name,
					code,
					timeColor,
					bar,
					rem,
				)
			}
			fmt.Println("\n\033[2mPress Ctrl+C to exit\033[0m")
		}
	}
}

func handleShow(vaultPath string, args []string) error {
	showCmd := flag.NewFlagSet("show", flag.ContinueOnError)
	copyOpt := showCmd.Bool("c", false, "Copy code to clipboard")
	copyLongOpt := showCmd.Bool("copy", false, "Copy code to clipboard")
	secretOpt := showCmd.Bool("secret", false, "Show raw secret key instead of TOTP code")

	positional, err := parseFlagsAndPositional(showCmd, args)
	if err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("missing account name. Usage: cfa show <name>")
	}
	query := positional[0]

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	// Smart account matching
	var matches []VaultEntry
	for _, entry := range entries {
		if strings.EqualFold(entry.Name, query) {
			matches = []VaultEntry{entry} // Exact match takes precedence
			break
		}
		if strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query)) {
			matches = append(matches, entry)
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf("no accounts match query '%s'", query)
	}
	if len(matches) > 1 {
		fmt.Fprintln(os.Stderr, "Multiple matches found:")
		for _, m := range matches {
			fmt.Fprintf(os.Stderr, "  - %s\n", m.Name)
		}
		return fmt.Errorf("ambiguous query '%s', please be more specific", query)
	}

	target := matches[0]

	if *secretOpt {
		// Output raw secret
		fmt.Println(target.Secret)
		return nil
	}

	code, err := GenerateTOTP(target, time.Now())
	if err != nil {
		return err
	}

	// Print code
	fmt.Println(code)

	// Copy to clipboard if requested
	if *copyOpt || *copyLongOpt {
		if err := CopyToClipboard(code); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("\033[32mCopied code to clipboard!\033[0m")
	}

	return nil
}

func handleRemove(vaultPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing account name. Usage: cfa remove <name>")
	}
	query := args[0]

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	index := -1
	for i, entry := range entries {
		if strings.EqualFold(entry.Name, query) {
			index = i
			break
		}
	}

	if index == -1 {
		// Try substring match
		var matches []int
		for i, entry := range entries {
			if strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query)) {
				matches = append(matches, i)
			}
		}
		if len(matches) == 0 {
			return fmt.Errorf("no accounts match '%s'", query)
		}
		if len(matches) > 1 {
			fmt.Fprintln(os.Stderr, "Multiple matches found:")
			for _, idx := range matches {
				fmt.Fprintf(os.Stderr, "  - %s\n", entries[idx].Name)
			}
			return fmt.Errorf("ambiguous query '%s', please be more specific", query)
		}
		index = matches[0]
	}

	targetName := entries[index].Name
	fmt.Printf("Are you sure you want to permanently delete account '%s'? [y/N]: ", targetName)
	var resp string
	fmt.Scanln(&resp)
	resp = strings.ToLower(strings.TrimSpace(resp))
	if resp != "y" && resp != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	// Remove element from slice
	entries = append(entries[:index], entries[index+1:]...)

	if err := SaveVault(vaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully deleted account '%s'\033[0m\n", targetName)
	return nil
}

func handleRename(vaultPath string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("missing arguments. Usage: cfa rename <old_name> <new_name>")
	}
	oldName := args[0]
	newName := strings.TrimSpace(args[1])
	if newName == "" {
		return fmt.Errorf("new account name cannot be empty")
	}

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	index := -1
	for i, entry := range entries {
		if strings.EqualFold(entry.Name, oldName) {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("no account found named '%s'", oldName)
	}

	// Check if target name already exists
	for i, entry := range entries {
		if i != index && strings.EqualFold(entry.Name, newName) {
			return fmt.Errorf("an account named '%s' already exists", newName)
		}
	}

	actualOldName := entries[index].Name
	entries[index].Name = newName

	if err := SaveVault(vaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully renamed '%s' to '%s'\033[0m\n", actualOldName, newName)
	return nil
}

func handlePasswd(vaultPath string) error {
	currentPwd, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	// Validate the current password by loading the vault
	entries, err := LoadVault(vaultPath, currentPwd)
	if err != nil {
		return err
	}

	newPwd, err := GetMasterPassword("Set new master password: ", true)
	if err != nil {
		return err
	}

	if currentPwd == newPwd {
		return fmt.Errorf("new password is identical to the current one")
	}

	// Re-encrypt the vault entries with the new password
	if err := SaveVault(vaultPath, entries, newPwd); err != nil {
		return fmt.Errorf("failed to save vault with new password: %w", err)
	}

	fmt.Println("\033[32mSuccess: Master password successfully changed\033[0m")
	return nil
}

func handleExport(vaultPath string, args []string) error {
	exportCmd := flag.NewFlagSet("export", flag.ContinueOnError)
	outOpt := exportCmd.String("out", "", "Output JSON file path")

	if _, err := parseFlagsAndPositional(exportCmd, args); err != nil {
		return err
	}

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize vault to JSON: %w", err)
	}

	if *outOpt != "" {
		if err := os.WriteFile(*outOpt, jsonData, 0600); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}
		fmt.Printf("\033[32mSuccessfully exported %d entries to %s\033[0m\n", len(entries), *outOpt)
	} else {
		// Output to stdout
		fmt.Println(string(jsonData))
	}

	return nil
}

func handleImport(vaultPath string, args []string) error {
	importCmd := flag.NewFlagSet("import", flag.ContinueOnError)
	inOpt := importCmd.String("in", "", "Input JSON file path")

	if _, err := parseFlagsAndPositional(importCmd, args); err != nil {
		return err
	}

	password, err := getVaultPassword(vaultPath)
	if err != nil {
		return err
	}

	// Read imported JSON data
	var inputData []byte
	if *inOpt != "" {
		data, err := os.ReadFile(*inOpt)
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

	// Validate imported entries
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

	// Load existing entries
	existingEntries, err := LoadVault(vaultPath, password)
	if err != nil {
		return err
	}

	// Merge imported entries into existing ones
	mergedCount := 0
	addedCount := 0

	for _, imported := range importedEntries {
		found := false
		for i, existing := range existingEntries {
			if strings.EqualFold(existing.Name, imported.Name) {
				existingEntries[i] = imported // overwrite
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

	if err := SaveVault(vaultPath, existingEntries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccess: Imported %d entries (%d added, %d updated)\033[0m\n", len(importedEntries), addedCount, mergedCount)
	return nil
}

// CopyToClipboard writes text to system clipboard using native commands.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard utility found (please install wl-clipboard, xclip, or xsel)")
		}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := in.Write([]byte(text)); err != nil {
		return err
	}

	if err := in.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// parseFlagsAndPositional separates flags from positional arguments,
// parses the flags using fs, and returns the positional arguments.
func parseFlagsAndPositional(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagsArgs []string
	var positionalArgs []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagsArgs = append(flagsArgs, arg)
			// Check if flag needs a value.
			if !strings.Contains(arg, "=") {
				// Strip leading hyphens to find flag name
				name := strings.TrimLeft(arg, "-")
				// Check if this is a known boolean flag.
				f := fs.Lookup(name)
				isBool := false
				if f != nil {
					if boolVal, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolVal.IsBoolFlag() {
						isBool = true
					}
				}

				if !isBool && i+1 < len(args) {
					flagsArgs = append(flagsArgs, args[i+1])
					i++
				}
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
		i++
	}

	if err := fs.Parse(flagsArgs); err != nil {
		return nil, err
	}
	return positionalArgs, nil
}
