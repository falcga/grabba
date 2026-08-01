<img width="64" height="64" alt="logo" src="https://github.com/user-attachments/assets/d511fe74-e2cf-43eb-bfed-e993c70591eb" />

# grabba: Secret Scanner

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/falcga/grabba/actions/workflows/ci.yml/badge.svg)](https://github.com/falcga/grabba/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/falcga/grabba)](https://github.com/falcga/grabba/releases)

> **Grabba** is an entropy‑based secret scanner for Git repositories that prevents credential leaks before they enter your Git history.

---

## Overview

**Grabba** is a lightweight, high‑performance tool that detects potential secrets (passwords, API keys, tokens, private keys) in text files and Git repositories using **Shannon entropy analysis** and **pattern matching**.

It is designed for use in **pre‑commit hooks**, **CI/CD pipelines**, and ad‑hoc repository audits. All analysis is performed locally – no data leaves your environment.

In the **MITRE ATT&CK** framework, Grabba helps mitigate **Credential Access (TA0006)**, specifically **Unsecured Credentials: Credentials In Files (T1552.001)**.

---

## Features

- 🧠 **Shannon entropy calculation** – measures the randomness of strings to identify high‑entropy secrets
- 🔍 **10+ built‑in detection patterns** – AWS keys, JWT tokens, API keys, private keys, passwords, and more
- 🛡 **False‑positive filtering** – excludes common hashes (MD5/SHA), UUIDs, test data, and common words
- ⚡ **Parallel processing** – leverages all CPU cores for fast scanning of large codebases
- 💾 **Entropy caching** – speeds up repeated scans
- 📦 **Simple integrations** – pre‑commit hooks, GitHub Actions, Docker
- 🌐 **Automatic alphabet detection** – identifies the most relevant character set for entropy analysis
- 📊 **JSON output** – easy integration with other tools and reporting

---

## Installation

### Quick Install (Linux / macOS)

```bash
# Download the latest release binary
curl -L https://github.com/falcga/grabba/releases/latest/download/grabba-linux-amd64 -o grabba
chmod +x grabba
sudo mv grabba /usr/local/bin/
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/falcga/grabba.git
cd grabba

# Build the binary
make build

# Install to /usr/local/bin (optional)
sudo ./install.sh
```

### Using the Install Script

```bash
# Install Grabba binary to system
./install.sh
```

### Docker

```bash
# Pull the image
docker pull ghcr.io/falcga/grabba:latest

# Run the scan
docker run --rm -v $(pwd):/workspace ghcr.io/falcga/grabba:latest --repo /workspace
```

### Go Install

```bash
go install github.com/falcga/grabba/cmd/grabba@latest
```

---

## Usage

### Basic Commands

```bash
# Scan the current Git repository
grabba --repo .

# Scan a single file
grabba --file .env

# Scan the entire Git history
grabba --git-history

# Output results in JSON
grabba --repo . --json

# Save results to a file
grabba --repo . --output results.json
```

### Advanced Usage

```bash
# Custom thresholds
grabba --repo . --threshold 5.0 --min-length 12 --confidence 0.8

# Scan specific file extensions
grabba --repo . --extensions ".go,.py,.js,.env,.yml,.yaml"

# Exclude directories
grabba --repo . --exclude "vendor,test,examples"

# Limit file size
grabba --repo . --max-size 524288  # 512KB
```

---

## Command Line Arguments

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--file` | Analyze a specific file | — |
| `--repo` | Path to Git repository | `.` |
| `--threshold` | Minimum entropy (bits) | `4.5` |
| `--min-length` | Minimum secret length | `8` |
| `--max-length` | Maximum secret length | `256` |
| `--confidence` | Minimum confidence (0‑1) | `0.6` |
| `--json` | Output in JSON format | `false` |
| `--output` | Save results to file | — |
| `--git-history` | Analyze Git history | `false` |
| `--extensions` | Comma‑separated file extensions | `.py,.js,.go,.java,...` |
| `--exclude` | Comma‑separated directories to exclude | `.git,node_modules,...` |
| `--max-size` | Maximum file size in bytes | `1 MB` |
| `--version` | Show version | `false` |

---

## Examples

### 1. Scan a Repository with Stricter Thresholds

```bash
grabba --repo . --threshold 5.0 --min-length 12 --confidence 0.8
```

### 2. Scan a Specific File

```bash
grabba --file .env
```

### 3. Pre‑commit Hook

Add to `.git/hooks/pre-commit`:

```bash
#!/bin/sh
grabba --repo . --json --output /dev/null || exit 1
```

### 4. GitHub Actions Step

```yaml
- name: Scan for secrets with Grabba
  uses: falcga/grabba@v1
  with:
    args: --repo . --json --output reports/grabba.json
```

### 5. Scan and Generate JSON Report

```bash
grabba --repo . --json --output secrets_report.json
```

---

## CI/CD Integration

### Automated Workflow Installation

The repository includes `install_cicd.sh` which copies ready‑to‑use GitHub Actions workflows (CI, pre‑commit, release) into your project:

```bash
./install_cicd.sh /path/to/your/repo
```

After installation, every push/pull request will automatically run:
- Linters (golangci‑lint)
- Tests
- Binary build
- Secret scanning with Grabba

### GitHub Actions Workflow Example

```yaml
name: Secret Scan

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run Grabba
        uses: falcga/grabba@v1
        with:
          args: --repo . --json --output secrets.json
      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: secrets-report
          path: secrets.json
```

### Pre-commit Hook Setup

```bash
# Install pre-commit
pip install pre-commit
pre-commit install

# Run manually
pre-commit run --all-files
```

---

## Development

### Prerequisites

- Go 1.21 or higher
- Make
- Git

### Build

```bash
# Build the binary
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run linters
make lint

# Format code
make fmt

# Clean build artifacts
make clean
```

### Project Structure

```
grabba/
├── cmd/
│   └── grabba/
│       └── main.go          # CLI entry point
├── internal/
│   └── analyzer/
│       └── analyzer.go      # Core entropy analysis logic
├── .github/
│   └── workflows/
│       ├── ci.yml           # CI pipeline
│       ├── pre-commit.yml   # Pre-commit checks
│       └── release.yml      # Release automation
├── Makefile                 # Build commands
├── Dockerfile               # Docker build
├── go.mod                   # Go module
├── install.sh               # Installation script
├── install_cicd.sh          # CI/CD setup script
└── README.md               # This file
```

### Testing

```bash
# Run all tests with race detector
make test

# Run benchmarks
make bench

# Generate coverage report
make coverage
```

### Release

```bash
# Create a new release tag
git tag -a v1.0.0 -m "First stable release"
git push origin v1.0.0
```

GitHub Actions will automatically build binaries for all platforms and attach them to the release.

---

## Comparison with Other Tools

| Tool | Language | Detection Mechanisms | Entropy | Live Verification | Baseline Support | Primary Use Case |
|------|----------|-----------------------|---------|-------------------|------------------|------------------|
| **Gitleaks** | Go | Regex + Entropy | ✅ | ❌ | ✅ (toml) | Pre‑commit & CI/CD |
| **TruffleHog** | Go | Regex + Entropy + API | ✅ | ✅ | ✅ | Deep auditing, Enterprise CI/CD |
| **detect-secrets** | Python | Entropy + Plugins | ✅ (primary) | Limited | ✅ (baseline) | Large legacy codebases |
| **git-secrets** | Shell/egrep | Pure Regex | ❌ | ❌ | ❌ | Lightweight AWS‑focused repos |
| **Infisical CLI** | Go/JS | Regex + Entropy + Keywords | ✅ (threshold 3.5) | ❌ | ✅ (leaks-report) | Secret management ecosystem |
| **Grabba** (ours) | Go | Regex + Entropy + FP filtering | ✅ (flexible) | ❌ | Planned | Lightweight, fast, CI/CD |

---

## Detection Mechanisms

### 1. **Signature‑based Analysis (Regular Expressions)**

- Detects known provider token patterns (AWS, GitHub, Slack, etc.)
- Low false‑positive rate for structured tokens
- Cannot identify custom or internal secrets

### 2. **Shannon Entropy Analysis**

The entropy calculation measures the randomness of a string:

- **High entropy (>4.5 bits/char)**: cryptographic keys, tokens, strong passwords
- **Low entropy**: source code, natural language, placeholders (e.g., `password123`)

Grabba uses a dynamic alphabet detection to choose the most appropriate character set for entropy calculation, improving accuracy.

### 3. **False‑Positive Filtering**

To minimise noise, Grabba automatically ignores:
- Common hash formats (MD5, SHA‑1, SHA‑256, SHA‑512)
- UUIDs
- Strings consisting solely of digits
- Test/example/demo data
- Common words like `password`, `secret`, `token`, etc.

This multi‑layered filtering ensures that only genuinely suspicious strings are reported.

---

## Mitigation Strategy

| Phase | Action | Tool |
|-------|--------|------|
| **Prevention** | Block commits with secrets | Grabba (pre‑commit) |
| **Detection** | Scan pull requests | Grabba (CI) |
| **Remediation** | Rewrite history and rotate keys | `git filter-repo` + Provider API |
| **Monitoring** | Continuous scanning | GitHub Advanced Security / Grabba (cron) |

---

## License

This project is licensed under the **MIT License**
