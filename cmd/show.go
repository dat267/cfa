package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ShowCmd struct {
	Name   string `arg:"" required:"" help:"Account name"`
	Copy   bool   `short:"c" help:"Copy the generated code to the clipboard"`
	Secret bool   `help:"Show the raw Base32 secret key instead of the code"`
}

func (c *ShowCmd) Run(vaultPath VaultPath) error {
	query := c.Name

	entries, _, err := vaultPath.Open()
	if err != nil {
		return err
	}

	var matches []VaultEntry
	for _, entry := range entries {
		if strings.EqualFold(entry.Name, query) {
			matches = []VaultEntry{entry}
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

	if c.Secret {
		fmt.Println(target.Secret)
		return nil
	}

	code, err := generateTOTP(target, time.Now())
	if err != nil {
		return err
	}

	fmt.Println(code)

	if c.Copy {
		if err := copyToClipboard(code); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println(colorGreen + "Copied code to clipboard!" + colorReset)
	}

	return nil
}

func copyToClipboard(text string) error {
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
