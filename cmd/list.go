package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// ListCommand represents the command to list all credentials.
type ListCommand struct {
	fs        *flag.FlagSet
	vaultPath string
	liveOpt   bool
}

func NewListCommand(vaultPath string) *ListCommand {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	c := &ListCommand{fs: fs, vaultPath: vaultPath}
	fs.BoolVar(&c.liveOpt, "live", false, "Run interactive live dashboard")
	return c
}

func (c *ListCommand) Name() string { return "list" }
func (c *ListCommand) Description() string {
	return "Display the current and next TOTP codes for all accounts"
}
func (c *ListCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *ListCommand) Run(args []string) error {
	password, err := getVaultPassword(c.vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(c.vaultPath, password)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No accounts found. Add one with: cfa add <name>")
		return nil
	}

	// Run interactive dashboard if --live is explicitly passed and output is TTY
	if c.liveOpt && term.IsTerminal(int(os.Stdout.Fd())) {
		runLiveView(entries)
		return nil
	}

	// Default: Output static list showing current and next codes
	t := time.Now()
	fmt.Printf("%-30s %-12s %-12s %-5s %s\n", "Account", "Current Code", "Next Code", "Rem", "Parameters")
	fmt.Println(strings.Repeat("-", 75))
	for _, entry := range entries {
		currentCode, err := GenerateTOTP(entry, t)
		if err != nil {
			currentCode = "ERROR"
		}

		period := entry.Period
		if period == 0 {
			period = 30
		}
		rem := int(period) - int(t.Unix()%int64(period))

		nextTime := t.Add(time.Duration(rem) * time.Second)
		nextCode, err := GenerateTOTP(entry, nextTime)
		if err != nil {
			nextCode = "ERROR"
		}

		fmt.Printf("%-30s %-12s %-12s %2ds  %s (%d digits)\n",
			entry.Name, currentCode, nextCode, rem, entry.Algorithm, entry.Digits)
	}

	return nil
}

func runLiveView(entries []VaultEntry) {
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
			fmt.Print("\033[H") // Move cursor to top-left

			t := time.Now()
			fmt.Printf("\033[1;36m=== MFA Code Generator (cfa) ===\033[0m  Local Time: %s\n\n", t.Format("15:04:05"))
			fmt.Printf("\033[1m%-30s %-12s %-12s %-30s\033[0m\n", "Account", "Current", "Next", "Time Remaining")
			fmt.Println(strings.Repeat("-", 85))

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

				nextTime := t.Add(time.Duration(rem) * time.Second)
				nextCode, err := GenerateTOTP(entry, nextTime)
				if err != nil {
					nextCode = "ERROR"
				}

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

				fmt.Printf("%-30s \033[1;32m%-12s\033[0m \033[90m%-12s\033[0m %s[%s] %2ds remaining\033[0m\n",
					entry.Name,
					code,
					nextCode,
					timeColor,
					bar,
					rem,
				)
			}
			fmt.Println("\n\033[2mPress Ctrl+C to exit\033[0m")
		}
	}
}
