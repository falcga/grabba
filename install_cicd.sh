#!/bin/bash
set -e

GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo -e "${GREEN}Grabba CI/CD Installer${NC}"
echo "==========================="

TARGET_DIR="${1:-.}"
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Target directory '$TARGET_DIR' does not exist."
    exit 1
fi

cd "$TARGET_DIR"

mkdir -p .github/workflows

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Copying workflow files from $SOURCE_DIR/.github/workflows to .github/workflows/"
cp -v "$SOURCE_DIR/.github/workflows/ci.yml" .github/workflows/ 2>/dev/null || echo "ci.yml not found, skipping"
cp -v "$SOURCE_DIR/.github/workflows/pre-commit.yml" .github/workflows/ 2>/dev/null || echo "pre-commit.yml not found, skipping"
cp -v "$SOURCE_DIR/.github/workflows/release.yml" .github/workflows/ 2>/dev/null || echo "release.yml not found, skipping"

if [ -f "$SOURCE_DIR/.pre-commit-config.yaml" ]; then
    cp -v "$SOURCE_DIR/.pre-commit-config.yaml" ./
    echo "Copied .pre-commit-config.yaml"
fi

echo -e "${GREEN}✅ CI/CD workflows installed successfully to $TARGET_DIR${NC}"
echo "You may need to adjust the workflows to match your project."
