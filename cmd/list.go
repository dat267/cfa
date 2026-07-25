package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

type ListCmd struct {
	Live bool `help:"Run interactive live dashboard TUI instead of static list"`
}

func (c *ListCmd) Run(vaultPath VaultPath) error {
	entries, _, err := vaultPath.Open()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No accounts found. Add one with: cfa add <name>")
		return nil
	}

	if c.Live && term.IsTerminal(int(os.Stdout.Fd())) {
		runLiveView(entries)
		return nil
	}

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

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h\n")

	fmt.Print("\033[H\033[J")

	for {
		select {
		case <-sigChan:
			return
		case <-ticker.C:
			fmt.Print("\033[H")

			t := time.Now()
			fmt.Printf(colorCyan+"=== MFA Code Generator (cfa) ==="+colorReset+"  Local Time: %s\n\n", t.Format("15:04:05"))
			fmt.Printf(colorBold+"%-30s %-12s %-12s %-30s"+colorReset+"\n", "Account", "Current", "Next", "Time Remaining")
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

				timeColor := colorGreen
				if rem <= 5 {
					timeColor = colorRed
				} else if rem <= 10 {
					timeColor = colorYellow
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

				fmt.Printf("%-30s "+colorGreen+"%-12s"+colorReset+" "+colorDim+"%-12s"+colorReset+" %s[%s] %2ds remaining"+colorReset+"\n",
					entry.Name,
					code,
					nextCode,
					timeColor,
					bar,
					rem,
				)
			}
			fmt.Println("\n" + colorDim + "Press Ctrl+C to exit" + colorReset)
		}
	}
}
