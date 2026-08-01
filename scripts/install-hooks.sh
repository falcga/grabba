#!/bin/bash
set -e

echo "[ ] Installing pre-commit hooks..."

if ! command -v pre-commit &> /dev/null; then
    echo "[ ] Installing pre-commit..."
    pip install pre-commit
fi

pre-commit install

echo "[+] Pre-commit hooks installed successfully!"
