#!/bin/bash

set -e

REPO="GNURub/node-package-updater"
BINARY_NAME="npu"
INSTALL_DIR="/usr/local/bin"

OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
else
  echo "Unsupported architecture: $ARCH"
  exit 1
fi

LATEST_RELEASE=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
  echo "Failed to retrieve the latest release."
  exit 1
fi

echo "Latest release found: $LATEST_RELEASE"

BINARY_FILE="${BINARY_NAME}_${OS}_${ARCH}"
ASSET_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$BINARY_FILE"

echo "Downloading $ASSET_URL..."
curl -L "$ASSET_URL" -o "$BINARY_FILE"
chmod +x "$BINARY_FILE"

echo "Installing $BINARY_NAME to $INSTALL_DIR..."
sudo mv "$BINARY_FILE" "$INSTALL_DIR/$BINARY_NAME"

echo "$BINARY_NAME installed successfully in $INSTALL_DIR/"
echo "Installed version:"
"$INSTALL_DIR/$BINARY_NAME" version