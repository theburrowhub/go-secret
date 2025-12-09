# go-secrets

A beautiful terminal UI for managing GCP Secret Manager secrets, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

## Features

- 📁 **Folder-like navigation**: Secrets are organized into virtual folders based on a configurable separator
- 🔍 **Real-time filtering**: Quickly find secrets with instant search
- 🔐 **Version management**: View, reveal, and add new versions to secrets
- 📋 **Code generation**: Generate code snippets for common use cases (bash, helmfile, kyverno, etc.)
- ⚙️ **Configurable**: Store settings in a YAML config file
- 🎨 **Beautiful UI**: Modern terminal interface with keyboard shortcuts (nano-style footer)

## Installation

```bash
go install github.com/jamuriano/go-secrets@latest
```

Or build from source:

```bash
git clone https://github.com/jamuriano/go-secrets.git
cd go-secrets
go build -o go-secrets .
```

## Usage

```bash
# Run with a project ID
go-secrets -project my-gcp-project

# Or use the shorthand
go-secrets -p my-gcp-project

# Run without arguments to be prompted for project ID
go-secrets
```

### Authentication

The application uses Google Cloud's Application Default Credentials (ADC). Make sure you're authenticated:

```bash
gcloud auth application-default login
```

Or set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable to point to a service account key file.

## Keyboard Shortcuts

### List View

| Key | Action |
|-----|--------|
| `↑/k` | Move cursor up |
| `↓/j` | Move cursor down |
| `Enter` | Open folder/secret |
| `Backspace` | Go back to parent folder |
| `/` | Filter secrets |
| `n` | Create new secret |
| `d` | Delete secret |
| `Ctrl+R` | Refresh list |
| `Ctrl+P` | **Quick project switch** |
| `Ctrl+S` | Open settings menu |
| `q` | Quit |

### Quick Project Switch (`Ctrl+P`)

Available from **any screen**, press `Ctrl+P` to quickly switch between GCP projects:

- **All used projects are saved** - no limit on history
- Type to filter existing projects or **add a new project ID**
- Use `↑/↓` to select from saved projects
- Press `Enter` to switch (or add new)
- Press `Esc` to cancel

The selector shows:
- `➕ Add new project: "xxx"` - when typing a new project ID
- `➡️ Switch to: "xxx"` - when typing an existing project
- `✓` marks the current project

### Settings Menu

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate menu |
| `Enter` | Select option |
| `Esc` | Back to list |

#### Settings Sections

- **📋 Basic Settings**: Edit Project ID and folder separator
- **📝 Code Templates**: Manage code generation templates (add, edit, delete)
- **🕐 Recent Projects**: View and switch between recent projects
- **💾 Save & Exit**: Save all changes and return to list

### Detail View

| Key | Action |
|-----|--------|
| `↑/k` | Select previous version |
| `↓/j` | Select next version |
| `r` | Reveal secret value |
| `a` | Add new version |
| `g` | Generate code snippet |
| `d` | Delete secret |
| `Esc` | Go back to list |
| `q` | Quit |

## Configuration

The configuration file is stored in the user's config directory:

- **macOS**: `~/Library/Application Support/go-secrets/config.yaml`
- **Linux**: `~/.config/go-secrets/config.yaml`
- **Windows**: `%APPDATA%\go-secrets\config.yaml`

### Configuration Options

```yaml
# GCP Project ID
project_id: "my-project"

# Character used to create virtual folders in secret names
# e.g., with "/", "app/database/password" becomes: app > database > password
folder_separator: "/"

# All saved projects (no limit, most recent first)
recent_projects:
  - "project-1"
  - "project-2"
  - "project-3"

# Code templates for generation
templates:
  - title: "Bash Export"
    code: |
      export {{.SecretName}}=$(gcloud secrets versions access latest --secret="{{.FullSecretName}}" --project="{{.ProjectID}}")

  - title: "Helmfile secretRef"
    code: |
      - secretRef:
          name: {{.SecretName}}
          key: {{.FullSecretName}}

  - title: "Kyverno Policy"
    code: |
      apiVersion: kyverno.io/v1
      kind: ClusterPolicy
      metadata:
        name: require-secret-{{.SecretName}}
      spec:
        validationFailureAction: enforce
        rules:
        - name: check-secret
          match:
            resources:
              kinds:
              - Pod
          validate:
            message: "Secret {{.FullSecretName}} must be referenced"
            pattern:
              spec:
                containers:
                - env:
                  - valueFrom:
                      secretKeyRef:
                        name: "{{.SecretName}}"

  - title: "Go Client"
    code: |
      import secretmanager "cloud.google.com/go/secretmanager/apiv1"

      // Access the secret
      result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
          Name: "projects/{{.ProjectID}}/secrets/{{.FullSecretName}}/versions/latest",
      })
```

### Template Variables

When generating code, the following variables are available:

| Variable | Description |
|----------|-------------|
| `{{.SecretName}}` | Just the secret name (last part after separator) |
| `{{.FullSecretName}}` | The complete secret name |
| `{{.ProjectID}}` | The current GCP project ID |

### Managing Templates from the App

You can add, edit, and delete code templates directly from the application:

1. Press `Ctrl+S` to open Settings
2. Select "📝 Code Templates"
3. Use the following keys:
   - `n` - Create new template
   - `Enter/e` - Edit selected template
   - `d` - Delete selected template
4. In the template editor:
   - `Tab` - Switch between title and code fields
   - `Ctrl+S` - Save template
   - `Esc` - Cancel editing

## Required Permissions

The service account or user needs the following IAM roles:

- `roles/secretmanager.viewer` - To list and view secrets
- `roles/secretmanager.secretAccessor` - To access secret values
- `roles/secretmanager.admin` - To create, update, and delete secrets (optional)

## License

MIT

