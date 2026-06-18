package main

import (
	"cfa/cmd"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// Command defines the interface that each subcommand must satisfy.
type Command interface {
	Name() string
	Description() string
	FlagSet() *flag.FlagSet
	Run(args []string) error
}

var version = "cfa/development"

func main() {
	if err := runCLI(); err != nil {
		// Mitigate automated script brute forcing on the terminal by delaying the failure response
		if strings.Contains(err.Error(), "incorrect master password") {
			time.Sleep(2 * time.Second)
		}
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
  list                           Display the current and next TOTP codes for all accounts (default)
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
  -c, --copy, --copy-opt         Copy the generated code to the clipboard
  --secret                       Show the raw Base32 secret key instead of the code

Options for 'list':
  --live                         Run interactive live dashboard TUI instead of static list

Options for 'export':
  --out <file_path>              Output file path (default: stdout)

Options for 'import':
  --in <file_path>               Input file path (default: stdin)

Environment Variables:
  CFA_VAULT_PATH                 Override default vault file location (~/.config/cfa/vault.enc)
  CFA_PASSWORD                   Set master password non-interactively (useful for scripts)

Note: Running 'cfa' with no arguments will default to 'cfa list' (non-interactive list).
`)
}

func runCLI() error {
	vaultPath, err := cmd.DefaultVaultPath()
	if err != nil {
		return fmt.Errorf("failed to determine vault path: %w", err)
	}
	cmd.VaultPath = vaultPath

	subcommands := map[string]Command{
		"add":    cmd.NewAddCommand(),
		"export": cmd.NewExportCommand(),
		"import": cmd.NewImportCommand(),
		"init":   cmd.NewInitCommand(),
		"list":   cmd.NewListCommand(),
		"passwd": cmd.NewPasswdCommand(),
		"remove": cmd.NewRemoveCommand(),
		"rename": cmd.NewRenameCommand(),
		"show":   cmd.NewShowCommand(),
	}

	if len(os.Args) < 2 {
		// Default behavior: if vault exists, list. Otherwise, print usage.
		if _, err := os.Stat(vaultPath); err == nil {
			c := subcommands["list"]
			return c.Run([]string{})
		}
		printUsage()
		return nil
	}

	cmdName := os.Args[1]
	args := os.Args[2:]

	if cmdName == "help" || cmdName == "-h" || cmdName == "--help" {
		printUsage()
		return nil
	}

	if cmdName == "version" {
		fmt.Printf("cfa %s\n", version)
		return nil
	}

	c, ok := subcommands[cmdName]
	if !ok {
		return fmt.Errorf("unknown command '%s'. Run 'cfa help' for usage", cmdName)
	}

	// Parse flags using the subcommand's FlagSet
	positional, err := parseFlagsAndPositional(c.FlagSet(), args)
	if err != nil {
		return err
	}

	return c.Run(positional)
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
