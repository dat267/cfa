package main

import (
	"encoding/base32"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// DecodeQRCode reads a QR code image from disk and returns the decoded string content.
func DecodeQRCode(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Prepare BinaryBitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("failed to process image bitmap: %w", err)
	}

	// Decode QR code
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decode QR code: %w", err)
	}

	return result.GetText(), nil
}

// CleanSecret removes spaces and normalizes the Base32 secret key.
func CleanSecret(secret string) string {
	secret = strings.ReplaceAll(secret, " ", "")
	secret = strings.ReplaceAll(secret, "-", "")
	secret = strings.ToUpper(secret)
	return secret
}

// ValidateBase32 checks if the string is a valid Base32 encoded value.
func ValidateBase32(secret string) error {
	secret = CleanSecret(secret)
	// Base32 encoding characters: A-Z, 2-7.
	// We handle padding characters '=' optionally.
	secret = strings.TrimRight(secret, "=")
	_, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return fmt.Errorf("invalid Base32 format: %w", err)
	}
	return nil
}

// ParseOTPAuthURL parses an otpauth:// URL and populates a VaultEntry.
func ParseOTPAuthURL(uriStr string) (*VaultEntry, error) {
	u, err := url.Parse(uriStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme != "otpauth" {
		return nil, fmt.Errorf("unsupported scheme: %s (expected otpauth)", u.Scheme)
	}

	if u.Host != "totp" {
		return nil, fmt.Errorf("only TOTP is supported, found: %s", u.Host)
	}

	// Path usually looks like /Issuer:AccountName or /AccountName
	path := strings.TrimPrefix(u.Path, "/")
	
	// Query parameters
	q := u.Query()
	secret := q.Get("secret")
	if secret == "" {
		return nil, errors.New("missing secret query parameter")
	}
	secret = CleanSecret(secret)
	if err := ValidateBase32(secret); err != nil {
		return nil, err
	}

	issuer := q.Get("issuer")
	
	// If issuer is empty, try to parse it from the path (e.g. "GitHub:username")
	accountName := path
	if parts := strings.SplitN(path, ":", 2); len(parts) == 2 {
		if issuer == "" {
			issuer = parts[0]
		}
		accountName = parts[1]
	}

	name := accountName
	if issuer != "" {
		name = fmt.Sprintf("%s:%s", issuer, accountName)
	}

	// Defaults
	algo := "SHA1"
	if a := q.Get("algorithm"); a != "" {
		algo = strings.ToUpper(a)
	}

	digits := 6
	if d := q.Get("digits"); d != "" {
		if val, err := strconv.Atoi(d); err == nil {
			digits = val
		}
	}

	period := uint(30)
	if p := q.Get("period"); p != "" {
		if val, err := strconv.ParseUint(p, 10, 32); err == nil {
			period = uint(val)
		}
	}

	return &VaultEntry{
		Name:      name,
		Secret:    secret,
		Issuer:    issuer,
		Algorithm: algo,
		Digits:    digits,
		Period:    period,
	}, nil
}

// ParseAlgorithm maps string to otp.Algorithm
func ParseAlgorithm(algo string) (otp.Algorithm, error) {
	switch strings.ToUpper(algo) {
	case "SHA1", "":
		return otp.AlgorithmSHA1, nil
	case "SHA256":
		return otp.AlgorithmSHA256, nil
	case "SHA512":
		return otp.AlgorithmSHA512, nil
	default:
		return 0, fmt.Errorf("unsupported algorithm: %s (supported: SHA1, SHA256, SHA512)", algo)
	}
}

// ParseDigits maps int to otp.Digits
func ParseDigits(digits int) (otp.Digits, error) {
	switch digits {
	case 6, 0:
		return otp.DigitsSix, nil
	case 8:
		return otp.DigitsEight, nil
	default:
		return 0, fmt.Errorf("unsupported digits: %d (supported: 6, 8)", digits)
	}
}

// GenerateTOTP generates the 6/8-digit passcode for a VaultEntry at the given time.
func GenerateTOTP(entry VaultEntry, t time.Time) (string, error) {
	algo, err := ParseAlgorithm(entry.Algorithm)
	if err != nil {
		return "", err
	}

	digits, err := ParseDigits(entry.Digits)
	if err != nil {
		return "", err
	}

	period := entry.Period
	if period == 0 {
		period = 30
	}

	opts := totp.ValidateOpts{
		Period:    period,
		Skew:      1,
		Digits:    digits,
		Algorithm: algo,
	}

	// Secret keys might have lowercase or spaces (though we clean them on save).
	// Let's clean it again to be absolutely safe.
	secret := CleanSecret(entry.Secret)

	code, err := totp.GenerateCodeCustom(secret, t, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}

	return code, nil
}
