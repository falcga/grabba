#!/bin/bash
set -e

echo "[ ] Running tests..."

go test -v -race -coverprofile=coverage.out ./...

go tool cover -func=coverage.out

go tool cover -html=coverage.out -o coverage.html

echo "[+] Tests completed! Coverage report: coverage.html"
