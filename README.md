# 🔐 go-secret

```
 ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄       ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄ 
▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌     ▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌
▐░█▀▀▀▀▀▀▀▀▀ ▐░█▀▀▀▀▀▀▀█░▌     ▐░█▀▀▀▀▀▀▀▀▀ ▐░█▀▀▀▀▀▀▀▀▀ ▐░█▀▀▀▀▀▀▀▀▀ ▐░█▀▀▀▀▀▀▀█░▌▐░█▀▀▀▀▀▀▀▀▀  ▀▀▀▀█░█▀▀▀▀ 
▐░▌          ▐░▌       ▐░▌     ▐░▌          ▐░▌          ▐░▌          ▐░▌       ▐░▌▐░▌               ▐░▌     
▐░▌ ▄▄▄▄▄▄▄▄ ▐░▌       ▐░▌     ▐░█▄▄▄▄▄▄▄▄▄ ▐░█▄▄▄▄▄▄▄▄▄ ▐░▌          ▐░█▄▄▄▄▄▄▄█░▌▐░█▄▄▄▄▄▄▄▄▄      ▐░▌     
▐░▌▐░░░░░░░░▌▐░▌       ▐░▌     ▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░▌          ▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌     ▐░▌     
▐░▌ ▀▀▀▀▀▀█░▌▐░▌       ▐░▌      ▀▀▀▀▀▀▀▀▀█░▌▐░█▀▀▀▀▀▀▀▀▀ ▐░▌          ▐░█▀▀▀▀█░█▀▀ ▐░█▀▀▀▀▀▀▀▀▀      ▐░▌     
▐░▌       ▐░▌▐░▌       ▐░▌               ▐░▌▐░▌          ▐░▌          ▐░▌     ▐░▌  ▐░▌               ▐░▌     
▐░█▄▄▄▄▄▄▄█░▌▐░█▄▄▄▄▄▄▄█░▌      ▄▄▄▄▄▄▄▄▄█░▌▐░█▄▄▄▄▄▄▄▄▄ ▐░█▄▄▄▄▄▄▄▄▄ ▐░▌      ▐░▌ ▐░█▄▄▄▄▄▄▄▄▄      ▐░▌     
▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌     ▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░▌       ▐░▌▐░░░░░░░░░░░▌     ▐░▌     
 ▀▀▀▀▀▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀▀▀▀       ▀▀▀▀▀▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀▀▀▀  ▀         ▀  ▀▀▀▀▀▀▀▀▀▀▀       ▀      
```

**A security-first terminal UI for managing GCP Secret Manager secrets**, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)
![Security](https://img.shields.io/badge/Security-Hardened-red?style=flat&logo=shield)

---

## ✨ Features

- 📁 **Folder-like navigation**: Secrets organized into virtual folders based on a configurable separator
- 🔍 **Real-time filtering**: Quickly find secrets with instant search
- 🔐 **Version management**: View, reveal, and add new versions to secrets
- 📋 **Code generation**: Generate code snippets for common use cases (bash, helmfile, kyverno, etc.)
- ⚙️ **Configurable**: Store settings in a YAML config file
- 🎨 **Beautiful UI**: Modern terminal interface with Darcula theme and keyboard shortcuts

---

## 🛡️ Security Features

**go-secret** was built with security as a core principle. Every feature is designed to minimize the risk of secret exposure.

### 🔒 Secure Memory Handling

| Feature | Description |
|---------|-------------|
| **Byte-level storage** | Secrets are stored as `[]byte` instead of strings, allowing explicit memory zeroing |
| **Automatic zeroing** | Secret values are zeroed out immediately after display or copy |
| **No lingering data** | Memory is cleared when navigating away from secret views |

### 📋 Clipboard Protection

| Feature | Description |
|---------|-------------|
| **Auto-clear** | Clipboard is automatically cleared after configurable timeout (default: 30s) |
| **Visual countdown** | Status bar shows countdown until clipboard is cleared |
| **Secure library** | Uses actively maintained [`golang.design/x/clipboard`](https://github.com/golang-design/clipboard) |

### ⏰ Session Management

| Feature | Description |
|---------|-------------|
| **Inactivity timeout** | Session locks automatically after configurable inactivity (default: 15 min) |
| **Automatic lock** | Sensitive data is cleared from memory when session locks |
| **Quick unlock** | Press `Enter` or `Space` to unlock and resume |

### 📝 Audit Logging

| Feature | Description |
|---------|-------------|
| **Comprehensive logging** | All security-relevant events are logged with timestamps |
| **User identification** | Logs include GCP authenticated user email |
| **Structured JSON** | Machine-readable format for SIEM integration |
| **Log rotation** | Automatic rotation by size and age |
| **In-app viewer** | View audit logs directly from Security Settings |

**Events logged:**
- `SECRET_LIST`, `SECRET_ACCESS`, `SECRET_REVEAL`, `SECRET_COPY`
- `SECRET_CREATE`, `SECRET_DELETE`, `VERSION_ADD`
- `SESSION_START`, `SESSION_END`, `SESSION_LOCK`, `SESSION_UNLOCK`
- `CONFIG_CHANGE`, `PROJECT_SWITCH`, `CLIPBOARD_CLEAR`

### 🔐 File Security

| Feature | Description |
|---------|-------------|
| **Secure permissions** | Config files: `0600`, directories: `0700` |
| **Auto-fix** | Insecure permissions are automatically corrected on load |
| **No world-readable files** | Only the owner can access configuration and logs |

### 🛡️ Security Settings Menu

Access with `Ctrl+S` → "🔒 Security Settings":

```
📋 Clipboard
  ✓ Auto-clear: Enabled
  ⏱  Clear timeout: 30 seconds

📝 Audit Logging
  ✓ Audit logging: Enabled
  📅 Log retention: 90 days
  👁  View audit logs →

⏰ Session Security
  ✓ Lock on timeout: Enabled
  ⏱  Inactivity timeout: 15 minutes
```

---

## 🚀 Installation

### Quick Install (Recommended)

Download and install the latest release automatically:

```bash
curl -sSL https://raw.githubusercontent.com/theburrowhub/go-secret/main/install.sh | bash
```

### From Source

Clone the repository and run the install script:

```bash
git clone https://github.com/theburrowhub/go-secret.git
cd go-secret
./install.sh
```

### Using Go

```bash
go install github.com/theburrowhub/go-secret@latest
```

### Manual Build

```bash
git clone https://github.com/theburrowhub/go-secret.git
cd go-secret
make build
# Binary will be at ./go-secrets
```

## 📖 Usage

```bash
# Run with a project ID
go-secret -project my-gcp-project

# Or use the shorthand
go-secret -p my-gcp-project

# Run without arguments to be prompted for project ID
go-secret
```

### Authentication

The application uses Google Cloud's Application Default Credentials (ADC):

```bash
gcloud auth application-default login
```

Or set `GOOGLE_APPLICATION_CREDENTIALS` to point to a service account key file.

---

## ⌨️ Keyboard Shortcuts

### Global Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` | **Quick project switch** (from any screen) |
| `Ctrl+S` | Open settings menu |
| `Ctrl+C` | Quit |

### List View

| Key | Action |
|-----|--------|
| `↑/k` | Move cursor up |
| `↓/j` | Move cursor down |
| `Enter/l` | Open folder/secret |
| `Backspace/h` | Go back to parent folder |
| `/` | Filter secrets |
| `n` | Create new secret |
| `d` | Delete secret |
| `Ctrl+R` | Refresh list |

### Detail View

| Key | Action |
|-----|--------|
| `↑/k` | Select previous version |
| `↓/j` | Select next version |
| `r` | Reveal secret value |
| `c/y` | Copy to clipboard |
| `a` | Add new version |
| `g` | Generate code snippet |
| `d` | Delete secret |
| `Esc/h` | Go back to list |

### Reveal View

| Key | Action |
|-----|--------|
| `c/y` | Copy revealed value to clipboard |
| `Esc/r` | Hide value and go back |

---

## ⚙️ Configuration

Configuration is stored with **secure permissions** (`0600`):

- **macOS**: `~/Library/Application Support/go-secrets/config.yaml`
- **Linux**: `~/.config/go-secrets/config.yaml`
- **Windows**: `%APPDATA%\go-secrets\config.yaml`

### Full Configuration Example

```yaml
# GCP Project ID
project_id: "my-project"

# Folder separator for virtual folder navigation
folder_separator: "/"

# All saved projects (most recent first)
recent_projects:
  - "project-1"
  - "project-2"

# 🔒 Clipboard security settings
clipboard:
  auto_clear: true      # Automatically clear clipboard after copying
  timeout_seconds: 30   # Seconds before clipboard is cleared

# 📝 Audit logging settings
audit:
  enabled: true         # Enable comprehensive audit logging
  file_path: ""         # Custom path (empty = default location)
  max_size_mb: 10       # Max log file size before rotation
  max_age_days: 90      # Days to retain old logs

# ⏰ Session security settings
session:
  inactivity_timeout: 15  # Minutes of inactivity before lock (0 = disabled)
  lock_on_timeout: true   # Lock session on timeout

# Code generation templates
templates:
  - title: "Bash Export"
    code: |
      export {{.SecretName}}=$(gcloud secrets versions access latest --secret="{{.FullSecretName}}" --project="{{.ProjectID}}")
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `{{.SecretName}}` | Just the secret name (last part after separator) |
| `{{.FullSecretName}}` | The complete secret name |
| `{{.ProjectID}}` | The current GCP project ID |

---

## 🔑 Required Permissions

The service account or user needs the following IAM roles:

| Role | Purpose |
|------|---------|
| `roles/secretmanager.viewer` | List and view secrets |
| `roles/secretmanager.secretAccessor` | Access secret values |
| `roles/secretmanager.admin` | Create, update, and delete secrets (optional) |

---

## 📊 Audit Log Format

Audit logs are stored as JSON Lines format for easy parsing:

```json
{"timestamp":"2024-01-15T10:30:45Z","event_type":"SECRET_REVEAL","result":"SUCCESS","user":"user@example.com","project_id":"my-project","secret_name":"api-key","version":"1"}
{"timestamp":"2024-01-15T10:30:50Z","event_type":"SECRET_COPY","result":"SUCCESS","user":"user@example.com","project_id":"my-project","secret_name":"api-key","version":"1"}
{"timestamp":"2024-01-15T10:31:20Z","event_type":"CLIPBOARD_CLEAR","result":"SUCCESS","user":"user@example.com"}
```

**Log location:**
- **macOS**: `~/Library/Application Support/go-secrets/logs/audit.log`
- **Linux**: `~/.config/go-secrets/logs/audit.log`

---

## 🏗️ Development

```bash
# Build
make build

# Run tests
make test

# Run with race detector
make test-race

# Full check (fmt, vet, lint, test)
make check

# Build release binaries
make release
```

---

## 📜 License

MIT

---

<div align="center">

**Built with 💜 and security in mind**

[Report Bug](https://github.com/theburrowhub/go-secret/issues) · [Request Feature](https://github.com/theburrowhub/go-secret/issues)

</div>
