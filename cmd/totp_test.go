package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
)

func TestCleanSecret(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"gezd gnbv gy3t qojq", "GEZDGNBVGY3TQOJQ"},
		{"GEZD GNBV-GY3T qojq", "GEZDGNBVGY3TQOJQ"},
		{"", ""},
		{"abc123", "ABC123"},
	}
	for _, tt := range tests {
		if got := cleanSecret(tt.in); got != tt.want {
			t.Errorf("cleanSecret(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidateBase32(t *testing.T) {
	valid := []string{
		"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
		"gezd gnbv gy3t qojq",
		"JBSWY3DPEHPK3PXP",
	}
	for _, s := range valid {
		if err := validateBase32(s); err != nil {
			t.Errorf("validateBase32(%q) unexpected error: %v", s, err)
		}
	}

	invalid := []string{"", "NOT!BASE32", "12345", "####"}
	for _, s := range invalid {
		if err := validateBase32(s); err == nil {
			t.Errorf("validateBase32(%q) expected error, got nil", s)
		}
	}
}

func TestParseOTPAuthURL(t *testing.T) {
	uri := "otpauth://totp/GitHub:john?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=GitHub&algorithm=SHA256&digits=8&period=60"
	entry, err := parseOTPAuthURL(uri)
	if err != nil {
		t.Fatalf("parseOTPAuthURL: %v", err)
	}
	if entry.Name != "GitHub:john" {
		t.Errorf("Name = %q, want %q", entry.Name, "GitHub:john")
	}
	if entry.Issuer != "GitHub" {
		t.Errorf("Issuer = %q, want %q", entry.Issuer, "GitHub")
	}
	if entry.Algorithm != "SHA256" {
		t.Errorf("Algorithm = %q, want %q", entry.Algorithm, "SHA256")
	}
	if entry.Digits != 8 {
		t.Errorf("Digits = %d, want 8", entry.Digits)
	}
	if entry.Period != 60 {
		t.Errorf("Period = %d, want 60", entry.Period)
	}
}

func TestParseOTPAuthURL_Errors(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"wrong scheme", "http://totp/example?secret=X"},
		{"not totp", "otpauth://hotp/example?secret=X"},
		{"missing secret", "otpauth://totp/example"},
		{"invalid base32 secret", "otpauth://totp/example?secret=NOT_BASE32!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseOTPAuthURL(tt.uri); err == nil {
				t.Errorf("expected error for %q, got nil", tt.uri)
			}
		})
	}
}

func TestParseAlgorithm(t *testing.T) {
	valid := []struct {
		in   string
		want otp.Algorithm
	}{
		{"SHA1", otp.AlgorithmSHA1},
		{"", otp.AlgorithmSHA1},
		{"sha1", otp.AlgorithmSHA1},
		{"SHA256", otp.AlgorithmSHA256},
		{"SHA512", otp.AlgorithmSHA512},
	}
	for _, tt := range valid {
		got, err := parseAlgorithm(tt.in)
		if err != nil {
			t.Errorf("parseAlgorithm(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseAlgorithm(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	if _, err := parseAlgorithm("MD5"); err == nil {
		t.Error("parseAlgorithm(MD5) expected error, got nil")
	}
}

func TestParseDigits(t *testing.T) {
	valid := []struct {
		in   int
		want otp.Digits
	}{
		{6, otp.DigitsSix},
		{0, otp.DigitsSix},
		{8, otp.DigitsEight},
	}
	for _, tt := range valid {
		got, err := parseDigits(tt.in)
		if err != nil {
			t.Errorf("parseDigits(%d) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseDigits(%d) = %v, want %v", tt.in, got, tt.want)
		}
	}

	if _, err := parseDigits(7); err == nil {
		t.Error("parseDigits(7) expected error, got nil")
	}
}

// RFC 6238 test vector: ASCII secret "12345678901234567890".
const rfc6238Secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestGenerateTOTP_RFC6238(t *testing.T) {
	entry := VaultEntry{
		Secret:    rfc6238Secret,
		Algorithm: "SHA1",
		Digits:    8,
		Period:    30,
	}
	vectors := []struct {
		unix int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
	}
	for _, v := range vectors {
		got, err := generateTOTP(entry, time.Unix(v.unix, 0).UTC())
		if err != nil {
			t.Errorf("generateTOTP(t=%d) unexpected error: %v", v.unix, err)
			continue
		}
		if got != v.want {
			t.Errorf("generateTOTP(t=%d) = %q, want %q", v.unix, got, v.want)
		}
	}
}

func TestGenerateTOTP_CleansSecret(t *testing.T) {
	entry := VaultEntry{
		Secret:    "gezd gnbv gy3t qojq gezd gnbv gy3t qojq",
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
	}
	got, err := generateTOTP(entry, time.Unix(59, 0).UTC())
	if err != nil {
		t.Fatalf("generateTOTP: %v", err)
	}
	if got != "287082" {
		t.Errorf("generateTOTP with spaced lowercase secret = %q, want %q", got, "287082")
	}
}

func TestGenerateTOTP_UnsupportedAlgorithm(t *testing.T) {
	entry := VaultEntry{
		Secret:    rfc6238Secret,
		Algorithm: "MD5",
		Digits:    6,
		Period:    30,
	}
	if _, err := generateTOTP(entry, time.Unix(59, 0).UTC()); err == nil {
		t.Error("expected error for unsupported algorithm, got nil")
	}
}

func TestParseOTPAuthURL_IssuerFromPath(t *testing.T) {
	uri := "otpauth://totp/GitHub:john?secret=" + strings.TrimRight(rfc6238Secret, "=")
	entry, err := parseOTPAuthURL(uri)
	if err != nil {
		t.Fatalf("parseOTPAuthURL: %v", err)
	}
	if entry.Issuer != "GitHub" {
		t.Errorf("Issuer = %q, want %q", entry.Issuer, "GitHub")
	}
	if entry.Name != "GitHub:john" {
		t.Errorf("Name = %q, want %q", entry.Name, "GitHub:john")
	}
}
