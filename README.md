<img width="64" height="64" alt="logo" src="https://github.com/user-attachments/assets/d511fe74-e2cf-43eb-bfed-e993c70591eb" />

# grabba: Secret Scanner

## Overview

This project is a secret scanning tool designed to prevent credential compromise in Git repositories before they enter the Git history.

The analysis is framed within the MITRE ATT&CK taxonomy under **Credential Access (TA0006)** and specifically addresses **Unsecured Credentials: Credentials In Files (T1552.001)**.

---

## Comparative Analysis Table

| Tool | Language | Detection Mechanisms | Entropy Support | Live Key Verification | Baseline Support | Primary Use Case |
|:-----|:---------|:---------------------|:----------------|:----------------------|:-----------------|:-----------------|
| **Gitleaks** | Go | Regex + Entropy | ✅ Configurable | ❌ | ✅ (.gitleaks.toml) | Standard for pre-commit & CI/CD |
| **TruffleHog** | Go | Regex + Entropy + API Verification | ✅ | ✅ | ✅ | Deep auditing, enterprise CI/CD |
| **detect-secrets** | Python | Entropy + Signature Plugins | ✅ (primary) | Limited (via plugins) | ✅ (.secrets.baseline) | Large legacy codebases |
| **git-secrets** | Shell/egrep | Pure Regex | ❌ | ❌ | ❌ (uses .gitallowed) | Lightweight AWS-focused repos |
| **Infisical CLI** | Go/JS | Regex + Entropy + Keywords | ✅ (threshold 3.5) | ❌ | ✅ (leaks-report.json) | Secret management ecosystem |

---

### Detection Mechanisms

#### 1. **Signature-based Analysis (Regular Expressions)**
- Detects known provider token patterns (AWS, GitHub, Slack, etc.)
- Low false positive rate for structured tokens
- Cannot identify custom/internal secrets

#### 2. **Shannon Entropy Analysis**

The entropy calculation determines the randomness of strings:

- **High entropy (>3.5-4.5 bits/char)**: Cryptographic keys, tokens, passwords
- **Low entropy**: Source code, natural language, placeholders (password123)

#### 3. **Baseline & Allowlist Management**
- Suppresses alerts on existing, known-good code
- Uses `.secrets.baseline` or `.gitleaks.toml`
- Focuses detection on **new** secrets only

---

## Mitigation Strategy

| Phase | Action | Tool |
|-------|--------|------|
| **Prevention** | Block commits with secrets | Gitleaks (pre-commit) |
| **Detection** | Scan pull requests | TruffleHog (CI) |
| **Remediation** | Rewrite history & rotate keys | `git filter-repo` + Provider API |
| **Monitoring** | Continuous scanning | GitHub Advanced Security / TruffleHog |

---


## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
