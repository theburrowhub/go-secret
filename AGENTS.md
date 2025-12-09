# AGENTS.md

This file contains information for AI agents working on this project.


### Communication preferences

- **Language for communication:** Spanish (Spain)
- **Language for code/comments/documentation:** English (USA)
- Direct and concise, dislikes long code explanations unless explicitly requested

### Programming style

- **Preferred paradigm:** Functional code
- **Methodology:** TDD (Test-Driven Development) when possible
- **Principles:**
  - Protect code against side effects
  - Do not reorganize file structure without explicit permission
  - If a solution fails 2 times, change the approach instead of insisting

### UX/UI preferences

- **Terminal interfaces:** Nano-style with keyboard shortcuts visible in the footer
- **Color theme:** Darcula (JetBrains)
- **Keyboard shortcuts:** Intuitive (e.g., `c` for copy, not `y`)
- **Footers:** Must show ALL available options on each screen
- **Navigation:** Support for vim keys (`j/k/h/l`) in addition to arrows

### Development tools

- **Makefiles:** Likes them well-structured with:
  - Colored output
  - Targets organized by category
  - Version/build information
  - Hot reload for development

### Commit conventions

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, no code change)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `chore`: Maintenance tasks (deps, build, etc.)

**Examples:**
```
feat(ui): add copy to clipboard in reveal view
fix(gcp): handle connection timeout gracefully
docs: update README with new shortcuts
refactor(config): simplify template storage
chore(deps): update bubble tea to v1.2.4
```

---

## About the project

### go-secrets

TUI (Terminal User Interface) application for managing GCP Secret Manager secrets.

### Tech stack

- **Language:** Go 1.21+
- **TUI Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Charm)
- **Styles:** [Lip Gloss](https://github.com/charmbracelet/lipgloss) (Charm)
- **GCP Client:** `cloud.google.com/go/secretmanager`
- **Configuration:** YAML (`gopkg.in/yaml.v3`)
- **Clipboard:** `github.com/atotto/clipboard`

### Architecture

```
go-secrets/
├── main.go                     # Main entry point
├── internal/
│   ├── config/
│   │   └── config.go           # Persistent YAML configuration
│   ├── gcp/
│   │   └── client.go           # GCP Secret Manager client
│   └── ui/
│       ├── model.go            # Main Bubble Tea model
│       ├── styles.go           # Darcula styles
│       └── keys.go             # Keybindings and footers
├── Makefile                    # Build, test, release
├── go.mod / go.sum
├── README.md
├── LICENSE                     # MIT
└── AGENTS.md                   # This file
```

### Main features

1. **Folder-like navigation:** Secrets organized by configurable separator
2. **Full CRUD:** Create, read, update, delete secrets
3. **Version management:** View, reveal, add versions
4. **Code generation:** Customizable templates for bash, helmfile, kyverno, etc.
5. **Quick project switch:** `Ctrl+P` from any screen
6. **In-app configuration:** `Ctrl+S` to manage settings

### Model views

| View | Description |
|------|-------------|
| `ViewProjectPrompt` | Initial prompt for project ID |
| `ViewList` | List of secrets/folders |
| `ViewDetail` | Secret detail with versions |
| `ViewCreate` | Create new secret |
| `ViewAddVersion` | Add version to a secret |
| `ViewDelete` | Delete confirmation |
| `ViewGenerate` | Code generation from templates |
| `ViewConfigMenu` | Configuration menu |
| `ViewConfigBasic` | Basic settings |
| `ViewConfigTemplates` | Template management |
| `ViewConfigTemplateEdit` | Template editor |
| `ViewConfigRecentProjects` | Recent projects |
| `ViewFilter` | Secret filtering |
| `ViewReveal` | Show secret value |
| `ViewProjectSwitch` | Quick project selector |

### Configuration

Location by platform:
- **macOS:** `~/Library/Application Support/go-secrets/config.yaml`
- **Linux:** `~/.config/go-secrets/config.yaml`
- **Windows:** `%APPDATA%\go-secrets\config.yaml`

### Global shortcuts

| Shortcut | Action | Available in |
|----------|--------|--------------|
| `Ctrl+P` | Quick project switch | All views |
| `Ctrl+C` | Exit | All views |
| `q` | Exit | Most views |

### Code conventions

- Bubble Tea messages as structs with `Msg` suffix
- Commands as functions returning `tea.Cmd`
- Update handlers as `update<View>(msg tea.KeyMsg)` methods
- Renderers as `view<View>() string` methods
- Footers defined in `keys.go` as `<View>Bindings()` functions

### Testing

```bash
make test          # Tests with race detector
make test-coverage # Tests with HTML report
make check         # fmt + vet + lint + test
```

### Build and release

```bash
make build    # Local build
make release  # Binaries for darwin/linux/windows (amd64/arm64)
make install  # Install to GOPATH/bin
```
