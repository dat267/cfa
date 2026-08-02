package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"
)

func getMasterPassword(prompt string, confirm bool, allowEnv bool) (string, error) {
	if allowEnv {
		if pwd := os.Getenv("CFA_PASSWORD"); pwd != "" {
			return pwd, nil
		}
	}

	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()

	pwd := string(bytePassword)
	if pwd == "" {
		return "", errors.New("password cannot be empty")
	}

	if confirm {
		fmt.Print("Confirm password: ")
		byteConfirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("failed to read confirmation password: %w", err)
		}
		fmt.Println()
		if pwd != string(byteConfirm) {
			return "", errors.New("passwords do not match")
		}
	}

	return pwd, nil
}

func confirmAction(format string, args ...interface{}) (bool, error) {
	fmt.Printf(format+" [y/N]: ", args...)
	var resp string
	_, err := fmt.Scanln(&resp)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "y" || resp == "yes", nil
}
