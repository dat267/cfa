#!/bin/bash
set -e

REPO="dat267/cfa"
BINARY_NAME="cfa"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux*)   OS="linux" ;;
    darwin*)  OS="darwin" ;;
    msys*|cygwin*|mingw*) OS="windows" ;;
    *)        echo "Error: Unsupported OS: $OS"; exit 1 ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    armv7*)        ARCH="arm" ;;
    *)             echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Form asset name
SUFFIX=""
[ "$OS" = "windows" ] && SUFFIX=".exe"
ASSET_NAME="${BINARY_NAME}-${OS}-${ARCH}${SUFFIX}"

echo "Fetching latest release information for $REPO..."
RELEASE_JSON=$(curl -s "https://api.github.com/repos/${REPO}/releases")

# Find the latest release tag
TAG=$(echo "$RELEASE_JSON" | grep -m 1 '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: Could not find any releases for $REPO."
    exit 1
fi

echo "Latest release tag: $TAG"

# Download URL
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET_NAME}"

echo "Downloading $ASSET_NAME from $DOWNLOAD_URL..."
TMP_DIR=$(mktemp -d)
curl -L -o "${TMP_DIR}/${BINARY_NAME}${SUFFIX}" "$DOWNLOAD_URL"

# Install location
if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$HOME/bin"
    mkdir -p "$INSTALL_DIR"
    mv "${TMP_DIR}/${BINARY_NAME}.exe" "${INSTALL_DIR}/${BINARY_NAME}.exe"
    echo "Successfully installed ${BINARY_NAME}.exe to ${INSTALL_DIR}/${BINARY_NAME}.exe"
    echo "Make sure ${INSTALL_DIR} is in your Windows PATH."
else
    # For Linux and Mac
    INSTALL_DIR="/usr/local/bin"
    chmod +x "${TMP_DIR}/${BINARY_NAME}"
    
    echo "Installing ${BINARY_NAME} to ${INSTALL_DIR}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo "Write permissions to ${INSTALL_DIR} required. Prompting for sudo..."
        sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi
    echo "Successfully installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
fi

# Clean up
rm -rf "$TMP_DIR"
