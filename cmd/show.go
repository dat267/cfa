package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)



// ShowCommand represents the command to fetch a single code.
type ShowCommand struct {
	fs        *flag.FlagSet
	copyOpt   bool
	secretOpt bool
}

func NewShowCommand() *ShowCommand {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	c := &ShowCommand{fs: fs}
	fs.BoolVar(&c.copyOpt, "c", false, "Copy code to clipboard")
	fs.BoolVar(&c.copyOpt, "copy", false, "Copy code to clipboard")
	fs.BoolVar(&c.secretOpt, "secret", false, "Show raw secret key instead of TOTP code")
	return c
}

func (c *ShowCommand) Name() string        { return "show" }
func (c *ShowCommand) Description() string { return "Show the current 6/8-digit code for a specific account" }
func (c *ShowCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *ShowCommand) Run(positional []string) error {
	if len(positional) < 1 {
		return fmt.Errorf("missing account name. Usage: cfa show <name>")
	}
	query := positional[0]

	password, err := getVaultPassword()
	if err != nil {
		return err
	}

	entries, err := LoadVault(VaultPath, password)
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

	if c.secretOpt {
		fmt.Println(target.Secret)
		return nil
	}

	code, err := GenerateTOTP(target, time.Now())
	if err != nil {
		return err
	}

	fmt.Println(code)

	if c.copyOpt {
		if err := CopyToClipboard(code); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("\033[32mCopied code to clipboard!\033[0m")
	}

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
