# cfa - Cryptographically Secure CLI MFA Code Generator

`cfa` is a lightweight, self-contained, and cryptographically secure CLI Time-Based One-Time Password (TOTP) generator written in Go. It allows you to manage and generate 6-digit or 8-digit MFA codes for all your online accounts directly from your terminal.

## Key Features

- 🔒 **Cryptographically Secure**: Secrets are encrypted at rest using **AES-256-GCM**. The encryption key is derived from your master password using **PBKDF2-HMAC-SHA256** with 600,000 iterations and a cryptographically secure random salt.
- 🖼️ **QR Code Image Scanning**: Automatically parse and extract MFA secrets from QR code images (supports PNG, JPEG, GIF formats) using a pure Go QR-decoder library.
- 📊 **Interactive Live Dashboard**: Launch `cfa` or `cfa list` without arguments to see a real-time updating table of your accounts, their current codes, and animated progress bars showing the time remaining for each token.
- 📋 **Clipboard Integration**: Instantly copy codes to your clipboard using the `-c` or `--copy` flag (supports `pbcopy`, `wl-copy`, `xclip`, and `xsel`).
- 🔍 **Smart Account Matching**: Quickly fetch codes using case-insensitive substring matching (e.g. `cfa show git` matches `GitHub:john`). Prompts you if the search is ambiguous.
- 🔧 **Zero External System Dependencies**: Compiled as a static Go binary. No GPG configuration, external databases, or heavy scripting dependencies required.
- 🚀 **Import/Export**: Easily backup or migrate your MFA vault using the `export` and `import` JSON subcommands.

---

## Build & Installation

Ensure you have [Go](https://go.dev/doc/install) installed (Go 1.21+ recommended).

1. Clone the repository and navigate inside:
   ```bash
   cd cfa
   ```

2. Compile the binary:
   ```bash
   go build -o cfa .
   ```

3. Move the binary to a directory in your PATH (e.g. `/usr/local/bin` or `~/bin`):
   ```bash
   mv cfa ~/bin/
   ```

---

## Getting Started

### 1. Initialize the Vault
Create a new secure vault. You will be prompted to set a master password:
```bash
cfa init
```
This writes an encrypted vault configuration file (default location: `~/.config/cfa/vault.enc`).

### 2. Add an Account
You can add accounts in multiple ways:

#### A. Scan a QR Code Image:
If you have a QR code image file downloaded from a service (e.g., GitHub, Google, AWS):
```bash
cfa add GitHub --qr ~/Downloads/github_qr.png
```
*If you omit the name argument, `cfa` will attempt to parse the issuer/account name directly from the QR code.*

#### B. Direct Key Entry (Manual):
If you only have the text Base32 secret key:
```bash
cfa add GitHub --secret "GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ"
```
*If you omit the `--secret` flag, you will be prompted to paste it securely without echoing.*

#### C. Custom TOTP Parameters:
`cfa` supports custom TOTP configs (e.g., 8-digit codes, custom period lengths, or different hashing algorithms):
```bash
cfa add MyService --secret "..." --digits 8 --period 60 --algo SHA256
```

---

## Usage Guide

### Display Interactive Dashboard
Running `cfa` with no arguments, or running `cfa list`, shows a live-updating TUI of all TOTP codes:
```bash
cfa
```
*Press `Ctrl+C` to exit.*

### Retrieve and Copy a Code
Get the code for a specific account. The search is case-insensitive and supports substrings:
```bash
cfa show github
```

Copy the code directly to your system clipboard:
```bash
cfa show github -c
```

### Scripting & Piping
If you want to use `cfa` in bash scripts or pipe outputs, use the `--static` option to disable terminal interactive rendering:
```bash
cfa list --static
```

### Delete or Rename Accounts
Remove an account:
```bash
cfa remove github
```
*Requires confirmation.*

Rename an account:
```bash
cfa rename github github_work
```

### Manage Password
Change the master password protecting the vault. All credentials will be re-encrypted:
```bash
cfa passwd
```

### Backup & Restore

#### Export:
Export all entries in raw decrypted JSON format:
```bash
cfa export --out ~/backup_vault.json
```
*Warning: Keep the exported file highly secure as it contains plaintext secrets.*

#### Import:
Import entries from a backup JSON file (merges new entries and updates existing ones):
```bash
cfa import --in ~/backup_vault.json
```

---

## Security Specifications

1. **Vault Encryption**: Standard **AES-256-GCM** authenticated symmetric encryption.
2. **Key Derivation**: **PBKDF2-HMAC-SHA256** with **600,000 iterations** (OWASP standard recommendation).
3. **Randomization**: A cryptographically secure random 32-byte salt and 12-byte nonce (using `crypto/rand`) are generated on every vault write.
4. **File Permissions**: The vault file is written with strict `0600` permissions (read/write access by the owner only).
5. **Memory Wiping**: User passwords entered in interactive prompts are held as byte slices and cleared from memory as soon as the key is derived.
6. **Automation-Friendly**: You can bypass interactive password prompts in scripts by setting the `CFA_PASSWORD` environment variable.

---

## License
Licensed under the MIT License.
