package cmd

import (
	"testing"
	"time"
)

func TestCleanSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"JBSW Y3DP EHPK 3PXP", "JBSWY3DPEHPK3PXP"},
		{"jbsw-y3dp-ehpk-3pxp", "JBSWY3DPEHPK3PXP"},
		{"  jbswy3dpehpk3pxp  ", "JBSWY3DPEHPK3PXP"},
	}

	for _, test := range tests {
		got := CleanSecret(test.input)
		if got != test.expected {
			t.Errorf("CleanSecret(%q) = %q; expected %q", test.input, got, test.expected)
		}
	}
}

func TestValidateBase32(t *testing.T) {
	valid := []string{
		"JBSWY3DPEHPK3PXP",
		"jbswy3dpehpk3pxp",
		"JBSWY3DPEHPK3PXP====",
		"MZXW6YTBOI",
	}

	for _, secret := range valid {
		if err := ValidateBase32(secret); err != nil {
			t.Errorf("ValidateBase32(%q) failed: %v", secret, err)
		}
	}

	invalid := []string{
		"JBSWY3DPEHPK3PX8", // '8' is not in Base32 alphabet
		"invalid-chars!",
		"123", // '1' is not in Base32 alphabet
	}

	for _, secret := range invalid {
		if err := ValidateBase32(secret); err == nil {
			t.Errorf("ValidateBase32(%q) succeeded, expected failure", secret)
		}
	}
}

func TestParseOTPAuthURL(t *testing.T) {
	uri := "otpauth://totp/GitHub:alice@gmail.com?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&algorithm=SHA256&digits=8&period=60"
	entry, err := ParseOTPAuthURL(uri)
	if err != nil {
		t.Fatalf("failed to parse valid OTP URI: %v", err)
	}

	if entry.Name != "GitHub:alice@gmail.com" {
		t.Errorf("expected Name GitHub:alice@gmail.com, got: %s", entry.Name)
	}
	if entry.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected Secret JBSWY3DPEHPK3PXP, got: %s", entry.Secret)
	}
	if entry.Issuer != "GitHub" {
		t.Errorf("expected Issuer GitHub, got: %s", entry.Issuer)
	}
	if entry.Algorithm != "SHA256" {
		t.Errorf("expected Algorithm SHA256, got: %s", entry.Algorithm)
	}
	if entry.Digits != 8 {
		t.Errorf("expected Digits 8, got: %d", entry.Digits)
	}
	if entry.Period != 60 {
		t.Errorf("expected Period 60, got: %d", entry.Period)
	}
}

func TestGenerateTOTP(t *testing.T) {
	entry := VaultEntry{
		Name:      "RFC6238-Test",
		Secret:    "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", // "12345678901234567890" in Base32
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
	}

	// Epoch 1111111111 -> 2005-03-18 01:58:31 UTC
	fixedTime := time.Unix(1111111111, 0)

	code, err := GenerateTOTP(entry, fixedTime)
	if err != nil {
		t.Fatalf("failed to generate TOTP: %v", err)
	}

	// Standard TOTP value for secret "12345678901234567890" at epoch 1111111111 is 050471
	expectedCode := "050471"
	if code != expectedCode {
		t.Errorf("expected TOTP code %s, got: %s", expectedCode, code)
	}
}
