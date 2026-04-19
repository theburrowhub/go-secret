package ui

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/clipboard"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

// View represents the current view state
type View int

const (
	ViewProjectPrompt View = iota
	ViewList
	ViewDetail
	ViewCreate
	ViewAddVersion
	ViewDelete
	ViewGenerate
	ViewConfig
	ViewConfigMenu
	ViewConfigBasic
	ViewConfigTemplates
	ViewConfigTemplateEdit
	ViewConfigRecentProjects
	ViewConfigSecurity
	ViewAuditLog
	ViewFilter
	ViewReveal
	ViewProjectSwitch
	ViewLocked
)

// FolderItem represents either a folder or a secret in the tree view
type FolderItem struct {
	Name       string
	FullPath   string
	IsFolder   bool
	Secret     *gcp.Secret
	Children   map[string]*FolderItem
	Depth      int
}

// Model is the main application model
type Model struct {
	// Config
	config *config.Config
	
	// GCP client
	client *gcp.Client
	ctx    context.Context
	
	// UI state
	view           View
	previousView   View
	width          int
	height         int
	styles         *Styles
	keys           KeyMap
	
	// List view state
	secrets        []gcp.Secret
	folderTree     *FolderItem
	currentPath    []string
	displayItems   []*FolderItem
	cursor         int
	listOffset     int
	filterText     string
	filterInput    textinput.Model
	
	// Detail view state
	selectedSecret *gcp.Secret
	versions       []gcp.SecretVersion
	versionCursor  int
	revealedValue  []byte // Stored as []byte for secure memory handling
	revealVersion  string
	
	// Create view state
	createInputs       []textinput.Model
	createFocus        int
	createLocationIdx  int    // -1 = add new, 0 = global, 1+ = index+1 in config.SecretLocations
	createAddingLoc    bool   // true when typing a new location
	createLocInput     textinput.Model
	createValueArea    textarea.Model  // textarea for multiline secrets
	createEditorMode   bool            // true = textarea, false = password input
	
	// Add version view state
	versionInput   textinput.Model
	
	// Generate view state
	templateCursor int
	generatedCode  string
	
	// Config view state
	configInputs      []textinput.Model
	configFocus       int
	configMenuCursor  int
	configMenuItems   []string
	
	// Template editing state
	templateListCursor  int
	editingTemplateIdx  int
	templateTitleInput  textinput.Model
	templateCodeArea    textarea.Model
	templateFocus       int
	isNewTemplate       bool
	
	// Recent projects state
	recentProjectsCursor int
	
	// Security settings state
	securityCursor int
	
	// Audit log viewer state
	auditLogLines  []string
	auditLogOffset int
	
	// Project switch state
	projectSwitchCursor   int
	projectSwitchInput    textinput.Model
	projectSwitchPrevView View
	
	// Delete confirmation
	deleteConfirm  bool
	
	// Status message
	statusMsg      string
	statusErr      bool
	
	// Viewport for scrolling
	viewport       viewport.Model
	
	// Loading state
	loading        bool
	loadingMsg     string
	
	// Clipboard auto-clear state
	clipboardClearAt time.Time
	clipboardActive  bool
	
	// Audit logger
	auditLogger *audit.Logger
	
	// Session management state
	lastActivity    time.Time
	sessionLocked   bool
	lockedPrevView  View
}

// Messages
type secretsLoadedMsg struct {
	secrets []gcp.Secret
	err     error
}

type versionsLoadedMsg struct {
	versions []gcp.SecretVersion
	err      error
}

type secretCreatedMsg struct {
	name string
	err  error
}

type secretDeletedMsg struct {
	name string
	err  error
}

type versionAddedMsg struct {
	version *gcp.SecretVersion
	err     error
}

type secretValueMsg struct {
	secretName string
	value      []byte
	version    string
	err        error
}

type secretCopiedMsg struct {
	secretName string
	version    string
	err        error
}

type clientInitializedMsg struct {
	client *gcp.Client
	err    error
}

type clipboardClearMsg struct{}

type clipboardTickMsg time.Time

type sessionTimeoutMsg time.Time

// NewModel creates a new application model
func NewModel(cfg *config.Config, projectID string) Model {
	styles := NewStyles()
	keys := DefaultKeyMap()
	
	// Initialize filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Type to filter..."
	filterInput.CharLimit = 100
	
	// Initialize create inputs
	createInputs := make([]textinput.Model, 2)
	createInputs[0] = textinput.New()
	createInputs[0].Placeholder = "secret-name"
	createInputs[0].CharLimit = 255
	createInputs[1] = textinput.New()
	createInputs[1].Placeholder = "secret value"
	createInputs[1].CharLimit = 65536
	createInputs[1].EchoMode = textinput.EchoPassword
	
	// Initialize create location input
	createLocInput := textinput.New()
	createLocInput.Placeholder = "e.g. europe-west1, us-central1"
	createLocInput.CharLimit = 50
	
	// Initialize create value textarea (for multiline secrets like PEM keys)
	createValueArea := textarea.New()
	createValueArea.Placeholder = "Paste secret value here...\n(supports multiline, e.g. PEM keys)"
	createValueArea.CharLimit = 65536
	createValueArea.SetWidth(50)
	createValueArea.SetHeight(6)
	
	// Initialize version input
	versionInput := textinput.New()
	versionInput.Placeholder = "new secret value"
	versionInput.CharLimit = 65536
	versionInput.EchoMode = textinput.EchoPassword
	
	// Initialize config inputs
	configInputs := make([]textinput.Model, 2)
	configInputs[0] = textinput.New()
	configInputs[0].Placeholder = "project-id"
	configInputs[0].SetValue(cfg.ProjectID)
	configInputs[1] = textinput.New()
	configInputs[1].Placeholder = "/"
	configInputs[1].CharLimit = 5
	configInputs[1].SetValue(cfg.FolderSeparator)
	
	// Initialize template inputs
	templateTitleInput := textinput.New()
	templateTitleInput.Placeholder = "Template title"
	templateTitleInput.CharLimit = 100
	
	templateCodeArea := textarea.New()
	templateCodeArea.Placeholder = "Template code...\nUse {{.SecretName}}, {{.FullSecretName}}, {{.ProjectID}}"
	templateCodeArea.CharLimit = 4096
	templateCodeArea.SetWidth(60)
	templateCodeArea.SetHeight(8)
	
	// Config menu items
	configMenuItems := []string{
		"📋 Basic Settings",
		"📝 Code Templates",
		"🕐 Recent Projects",
		"🔒 Security Settings",
		"💾 Save & Exit",
	}
	
	// Project switch input
	projectSwitchInput := textinput.New()
	projectSwitchInput.Placeholder = "Enter project ID or select from list..."
	projectSwitchInput.CharLimit = 100
	
	// Determine initial view
	initialView := ViewList
	if projectID == "" && cfg.ProjectID == "" {
		initialView = ViewProjectPrompt
		configInputs[0].Focus()
	} else if projectID != "" {
		cfg.ProjectID = projectID
	}
	
	// Initialize audit logger
	auditCfg := audit.Config{
		Enabled:    cfg.Audit.Enabled,
		FilePath:   cfg.Audit.FilePath,
		MaxSizeMB:  cfg.Audit.MaxSizeMB,
		MaxAgeDays: cfg.Audit.MaxAgeDays,
	}
	auditLogger, _ := audit.NewLogger(auditCfg)
	
	return Model{
		config:             cfg,
		ctx:                context.Background(),
		view:               initialView,
		styles:             styles,
		keys:               keys,
		filterInput:        filterInput,
		createInputs:       createInputs,
		createLocInput:     createLocInput,
		createLocationIdx:  0, // 0 = global
		createValueArea:    createValueArea,
		createEditorMode:   false,
		versionInput:       versionInput,
		configInputs:       configInputs,
		templateTitleInput: templateTitleInput,
		templateCodeArea:   templateCodeArea,
		configMenuItems:    configMenuItems,
		projectSwitchInput: projectSwitchInput,
		folderTree:         &FolderItem{Children: make(map[string]*FolderItem)},
		currentPath:        []string{},
		loading:            initialView == ViewList,
		loadingMsg:         "Loading secrets...",
		auditLogger:        auditLogger,
		lastActivity:       time.Now(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{sessionTimeoutTickCmd()}
	
	if m.view == ViewProjectPrompt {
		cmds = append(cmds, textinput.Blink)
	} else {
		cmds = append(cmds, m.initializeClient())
	}
	
	return tea.Batch(cmds...)
}

func (m Model) initializeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewClient(m.ctx, m.config.ProjectID)
		if err != nil {
			return clientInitializedMsg{err: err}
		}
		return clientInitializedMsg{client: client}
	}
}

func (m Model) loadSecrets() tea.Cmd {
	return func() tea.Msg {
		secrets, err := m.client.ListSecrets(m.ctx)
		return secretsLoadedMsg{secrets: secrets, err: err}
	}
}

func (m Model) loadVersions(secretName string) tea.Cmd {
	return func() tea.Msg {
		versions, err := m.client.ListSecretVersions(m.ctx, secretName)
		return versionsLoadedMsg{versions: versions, err: err}
	}
}

func (m Model) accessSecretVersion(secretName, version string) tea.Cmd {
	return func() tea.Msg {
		value, err := m.client.AccessSecretVersion(m.ctx, secretName, version)
		return secretValueMsg{secretName: secretName, value: value, version: version, err: err}
	}
}

func (m Model) createSecret(name string, value []byte, location string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.CreateSecret(m.ctx, name, nil, location)
		if err != nil {
			return secretCreatedMsg{name: name, err: err}
		}
		
		if len(value) > 0 {
			_, err = m.client.AddSecretVersion(m.ctx, name, value)
		}
		return secretCreatedMsg{name: name, err: err}
	}
}

func (m Model) deleteSecret(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteSecret(m.ctx, name)
		return secretDeletedMsg{name: name, err: err}
	}
}

func (m Model) addVersion(secretName string, value []byte) tea.Cmd {
	return func() tea.Msg {
		version, err := m.client.AddSecretVersion(m.ctx, secretName, value)
		return versionAddedMsg{version: version, err: err}
	}
}

func (m Model) copySecretValue(secretName, version string) tea.Cmd {
	return func() tea.Msg {
		value, err := m.client.AccessSecretVersion(m.ctx, secretName, version)
		if err != nil {
			return secretCopiedMsg{secretName: secretName, version: version, err: err}
		}
		err = clipboard.WriteText(string(value))
		return secretCopiedMsg{secretName: secretName, version: version, err: err}
	}
}

// clipboardTickCmd returns a command that ticks every second for countdown
func clipboardTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return clipboardTickMsg(t)
	})
}

// clearClipboardCmd returns a command that clears the clipboard
func clearClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		_ = clipboard.Clear()
		return clipboardClearMsg{}
	}
}

// sessionTimeoutTickCmd returns a command that checks for session timeout
func sessionTimeoutTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return sessionTimeoutMsg(t)
	})
}

// setRevealedValue securely sets the revealed secret value
func (m *Model) setRevealedValue(value []byte) {
	// Clear old value first
	m.clearRevealedValue()
	// Copy to new slice to avoid sharing memory with original
	m.revealedValue = make([]byte, len(value))
	copy(m.revealedValue, value)
}

// clearRevealedValue securely zeros and clears the revealed secret from memory
func (m *Model) clearRevealedValue() {
	if m.revealedValue != nil {
		// Zero out memory before releasing
		for i := range m.revealedValue {
			m.revealedValue[i] = 0
		}
		m.revealedValue = nil
	}
}

// getRevealedValueString returns the revealed value as string for display only
// Note: The returned string is still immutable in Go, but we minimize its lifetime
func (m *Model) getRevealedValueString() string {
	if m.revealedValue == nil {
		return ""
	}
	return string(m.revealedValue)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Track user activity for session timeout
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		m.lastActivity = time.Now()
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		
		// If session is locked, only handle unlock
		if m.sessionLocked {
			return m.updateLocked(msg)
		}
		
		// Global project switch (Ctrl+P) - available from most views
		if msg.String() == "ctrl+p" && m.view != ViewProjectPrompt && m.view != ViewProjectSwitch && m.view != ViewLocked {
			m.projectSwitchPrevView = m.view
			m.view = ViewProjectSwitch
			m.projectSwitchCursor = 0
			m.projectSwitchInput.SetValue("")
			m.projectSwitchInput.Focus()
			return m, textinput.Blink
		}
		
		// Handle view-specific keys
		switch m.view {
		case ViewProjectPrompt:
			return m.updateProjectPrompt(msg)
		case ViewList:
			return m.updateList(msg)
		case ViewDetail:
			return m.updateDetail(msg)
		case ViewCreate:
			return m.updateCreate(msg)
		case ViewAddVersion:
			return m.updateAddVersion(msg)
		case ViewDelete:
			return m.updateDelete(msg)
		case ViewGenerate:
			return m.updateGenerate(msg)
		case ViewConfig, ViewConfigMenu:
			return m.updateConfigMenu(msg)
		case ViewConfigBasic:
			return m.updateConfigBasic(msg)
		case ViewConfigTemplates:
			return m.updateConfigTemplates(msg)
		case ViewConfigTemplateEdit:
			return m.updateConfigTemplateEdit(msg)
		case ViewConfigRecentProjects:
			return m.updateConfigRecentProjects(msg)
		case ViewConfigSecurity:
			return m.updateConfigSecurity(msg)
		case ViewAuditLog:
			return m.updateAuditLog(msg)
		case ViewFilter:
			return m.updateFilter(msg)
		case ViewReveal:
			return m.updateReveal(msg)
		case ViewProjectSwitch:
			return m.updateProjectSwitch(msg)
		case ViewLocked:
			return m.updateLocked(msg)
		}
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width, msg.Height-6)
		
	case clientInitializedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			m.statusErr = true
			m.loading = false
			return m, nil
		}
		m.client = msg.client
		if m.auditLogger != nil {
			// Set the authenticated user in audit logger
			m.auditLogger.SetUser(msg.client.UserEmail())
			m.auditLogger.LogSessionStart(m.config.ProjectID)
		}
		m.loading = true
		m.loadingMsg = "Loading secrets..."
		return m, m.loadSecrets()
		
	case secretsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error loading secrets: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil {
				m.auditLogger.LogSecretList(m.config.ProjectID, 0, audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		m.secrets = msg.secrets
		m.buildFolderTree()
		m.updateDisplayItems()
		m.statusMsg = fmt.Sprintf("Loaded %d secrets", len(m.secrets))
		m.statusErr = false
		if m.auditLogger != nil {
			m.auditLogger.LogSecretList(m.config.ProjectID, len(m.secrets), audit.ResultSuccess, "")
		}
		
	case versionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error loading versions: %v", msg.err)
			m.statusErr = true
			return m, nil
		}
		m.versions = msg.versions
		
	case secretCreatedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error creating secret: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil {
				m.auditLogger.LogSecretCreate(m.config.ProjectID, msg.name, audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		m.statusMsg = "Secret created successfully"
		m.statusErr = false
		if m.auditLogger != nil {
			m.auditLogger.LogSecretCreate(m.config.ProjectID, msg.name, audit.ResultSuccess, "")
		}
		m.view = ViewList
		return m, m.loadSecrets()
		
	case secretDeletedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error deleting secret: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil {
				m.auditLogger.LogSecretDelete(m.config.ProjectID, msg.name, audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		m.statusMsg = "Secret deleted successfully"
		m.statusErr = false
		if m.auditLogger != nil {
			m.auditLogger.LogSecretDelete(m.config.ProjectID, msg.name, audit.ResultSuccess, "")
		}
		m.view = ViewList
		m.selectedSecret = nil
		return m, m.loadSecrets()
		
	case versionAddedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error adding version: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil && m.selectedSecret != nil {
				m.auditLogger.LogVersionAdd(m.config.ProjectID, m.selectedSecret.Name, "", audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Version %s added successfully", msg.version.Name)
		m.statusErr = false
		if m.auditLogger != nil && m.selectedSecret != nil {
			m.auditLogger.LogVersionAdd(m.config.ProjectID, m.selectedSecret.Name, msg.version.Name, audit.ResultSuccess, "")
		}
		m.view = ViewDetail
		return m, m.loadVersions(m.selectedSecret.Name)
		
	case secretValueMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error accessing secret: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil {
				m.auditLogger.LogSecretReveal(m.config.ProjectID, msg.secretName, msg.version, audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		// Securely store the secret value
		m.setRevealedValue(msg.value)
		// Zero original value from message
		for i := range msg.value {
			msg.value[i] = 0
		}
		m.revealVersion = msg.version
		m.view = ViewReveal
		if m.auditLogger != nil {
			m.auditLogger.LogSecretReveal(m.config.ProjectID, msg.secretName, msg.version, audit.ResultSuccess, "")
		}
		
	case secretCopiedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error copying: %v", msg.err)
			m.statusErr = true
			if m.auditLogger != nil {
				m.auditLogger.LogSecretCopy(m.config.ProjectID, msg.secretName, msg.version, audit.ResultFailure, msg.err.Error())
			}
			return m, nil
		}
		if m.auditLogger != nil {
			m.auditLogger.LogSecretCopy(m.config.ProjectID, msg.secretName, msg.version, audit.ResultSuccess, "")
		}
		// Start clipboard auto-clear timer if enabled
		if m.config.Clipboard.AutoClear && m.config.Clipboard.TimeoutSeconds > 0 {
			timeout := time.Duration(m.config.Clipboard.TimeoutSeconds) * time.Second
			m.clipboardClearAt = time.Now().Add(timeout)
			m.clipboardActive = true
			remaining := m.config.Clipboard.TimeoutSeconds
			m.statusMsg = fmt.Sprintf("📋 Copied! Auto-clear in %ds", remaining)
			m.statusErr = false
			return m, clipboardTickCmd()
		}
		m.statusMsg = "✓ Secret value copied to clipboard"
		m.statusErr = false
		
	case clipboardTickMsg:
		if !m.clipboardActive {
			return m, nil
		}
		remaining := time.Until(m.clipboardClearAt)
		if remaining <= 0 {
			return m, clearClipboardCmd()
		}
		m.statusMsg = fmt.Sprintf("📋 Clipboard will clear in %ds", int(remaining.Seconds()))
		m.statusErr = false
		return m, clipboardTickCmd()
		
	case clipboardClearMsg:
		m.clipboardActive = false
		m.statusMsg = "🔒 Clipboard cleared"
		m.statusErr = false
		if m.auditLogger != nil {
			m.auditLogger.LogClipboardClear()
		}
		
	case sessionTimeoutMsg:
		// Check if session timeout is enabled and if we should lock
		if m.config.Session.InactivityTimeout > 0 && m.config.Session.LockOnTimeout {
			timeout := time.Duration(m.config.Session.InactivityTimeout) * time.Minute
			if time.Since(m.lastActivity) > timeout && !m.sessionLocked {
				m.sessionLocked = true
				m.lockedPrevView = m.view
				m.view = ViewLocked
				// Clear sensitive data when locking
				m.clearRevealedValue()
				// Clear clipboard if active
				if m.clipboardActive {
					_ = clipboard.Clear()
					m.clipboardActive = false
				}
				if m.auditLogger != nil {
					m.auditLogger.LogSessionLock(m.config.ProjectID, "inactivity_timeout")
				}
			}
		}
		// Continue checking
		cmds = append(cmds, sessionTimeoutTickCmd())
	}
	
	return m, tea.Batch(cmds...)
}

func (m Model) updateProjectPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		projectID := m.configInputs[0].Value()
		if projectID == "" {
			m.statusMsg = "Project ID is required"
			m.statusErr = true
			return m, nil
		}
		m.config.ProjectID = projectID
		m.config.AddRecentProject(projectID)
		_ = m.config.Save()
		m.view = ViewList
		m.loading = true
		m.loadingMsg = "Connecting to GCP..."
		return m, m.initializeClient()
	case "q":
		return m, tea.Quit
	}
	
	var cmd tea.Cmd
	m.configInputs[0], cmd = m.configInputs[0].Update(msg)
	return m, cmd
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	
	// Calculate visible height for scrolling
	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			// Scroll up if cursor goes above visible area
			if m.cursor < m.listOffset {
				m.listOffset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.displayItems)-1 {
			m.cursor++
			// Scroll down if cursor goes below visible area
			if m.cursor >= m.listOffset+visibleHeight {
				m.listOffset = m.cursor - visibleHeight + 1
			}
		}
	case "g":
		// Go to top
		m.cursor = 0
		m.listOffset = 0
	case "G":
		// Go to bottom
		if len(m.displayItems) > 0 {
			m.cursor = len(m.displayItems) - 1
			if m.cursor >= visibleHeight {
				m.listOffset = m.cursor - visibleHeight + 1
			}
		}
	case "enter", "l":
		if len(m.displayItems) > 0 {
			item := m.displayItems[m.cursor]
			if item.IsFolder {
				m.currentPath = append(m.currentPath, item.Name)
				m.updateDisplayItems()
				m.cursor = 0
				m.listOffset = 0
			} else if item.Secret != nil {
				m.selectedSecret = item.Secret
				m.view = ViewDetail
				m.versionCursor = 0
				m.loading = true
				m.loadingMsg = "Loading versions..."
				return m, m.loadVersions(item.Secret.Name)
			}
		}
	case "backspace", "h", "esc":
		if len(m.currentPath) > 0 {
			m.currentPath = m.currentPath[:len(m.currentPath)-1]
			m.updateDisplayItems()
			m.cursor = 0
			m.listOffset = 0
		} else if m.filterText != "" {
			// Clear filter if at root
			m.filterText = ""
			m.filterInput.SetValue("")
			m.updateDisplayItems()
		}
	case "/":
		m.view = ViewFilter
		m.filterInput.Focus()
		return m, textinput.Blink
	case "n":
		m.view = ViewCreate
		m.createInputs[0].SetValue(strings.Join(m.currentPath, m.config.FolderSeparator))
		if len(m.currentPath) > 0 {
			m.createInputs[0].SetValue(m.createInputs[0].Value() + m.config.FolderSeparator)
		}
		m.createInputs[0].Focus()
		m.createFocus = 0
		// Initialize location selector: default to first saved location if any, else global
		if len(m.config.SecretLocations) > 0 {
			m.createLocationIdx = 1 // First saved location
		} else {
			m.createLocationIdx = 0 // Global
		}
		m.createAddingLoc = false
		m.createEditorMode = false
		m.createValueArea.SetValue("")
		return m, textinput.Blink
	case "d":
		if len(m.displayItems) > 0 && !m.displayItems[m.cursor].IsFolder {
			m.selectedSecret = m.displayItems[m.cursor].Secret
			m.view = ViewDelete
			m.deleteConfirm = false
		}
	case "ctrl+r":
		m.loading = true
		m.loadingMsg = "Refreshing..."
		return m, m.loadSecrets()
	case "ctrl+s":
		m.view = ViewConfigMenu
		m.configMenuCursor = 0
		return m, nil
	case "q":
		return m, tea.Quit
	}
	
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	
	switch msg.String() {
	case "up", "k":
		if m.versionCursor > 0 {
			m.versionCursor--
		}
	case "down", "j":
		if m.versionCursor < len(m.versions)-1 {
			m.versionCursor++
		}
	case "esc", "backspace", "h":
		m.view = ViewList
		m.selectedSecret = nil
		m.versions = nil
		// Securely clear secret from memory
		for i := range m.revealedValue {
			m.revealedValue[i] = 0
		}
		m.revealedValue = nil
	case "r":
		if len(m.versions) > 0 {
			version := m.versions[m.versionCursor]
			m.loading = true
			m.loadingMsg = "Accessing secret..."
			return m, m.accessSecretVersion(m.selectedSecret.Name, version.Name)
		}
	case "c", "y":
		if len(m.versions) > 0 {
			version := m.versions[m.versionCursor]
			m.loading = true
			m.loadingMsg = "Copying to clipboard..."
			return m, m.copySecretValue(m.selectedSecret.Name, version.Name)
		}
	case "a":
		m.view = ViewAddVersion
		m.versionInput.SetValue("")
		m.versionInput.Focus()
		return m, textinput.Blink
	case "g":
		m.view = ViewGenerate
		m.templateCursor = 0
		m.generatedCode = ""
	case "d":
		m.view = ViewDelete
		m.deleteConfirm = false
	case "q":
		return m, tea.Quit
	}
	
	return m, nil
}

func (m Model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle adding new location mode
	if m.createAddingLoc {
		switch msg.String() {
		case "enter":
			newLoc := m.createLocInput.Value()
			if newLoc != "" {
				m.config.AddSecretLocation(newLoc)
				_ = m.config.Save()
				// Point to the new location (index = len + 1 because 0 is global)
				m.createLocationIdx = len(m.config.SecretLocations)
			}
			m.createAddingLoc = false
			m.createLocInput.SetValue("")
			m.createLocInput.Blur()
			return m, nil
		case "esc":
			m.createAddingLoc = false
			m.createLocInput.SetValue("")
			m.createLocInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.createLocInput, cmd = m.createLocInput.Update(msg)
		return m, cmd
	}

	// Toggle editor mode with Ctrl+E
	if msg.String() == "ctrl+e" {
		m.createEditorMode = !m.createEditorMode
		if m.createEditorMode {
			// Copy value from input to textarea
			m.createValueArea.SetValue(m.createInputs[1].Value())
			m.createInputs[1].Blur()
			if m.createFocus == 1 {
				m.createValueArea.Focus()
			}
		} else {
			// Copy value from textarea to input
			m.createInputs[1].SetValue(m.createValueArea.Value())
			m.createValueArea.Blur()
			if m.createFocus == 1 {
				m.createInputs[1].Focus()
				return m, textinput.Blink
			}
		}
		return m, nil
	}

	// Field navigation: 0=name, 1=value, 2=location
	switch msg.String() {
	case "tab":
		// Blur current field
		if m.createFocus == 0 {
			m.createInputs[0].Blur()
		} else if m.createFocus == 1 {
			if m.createEditorMode {
				m.createValueArea.Blur()
			} else {
				m.createInputs[1].Blur()
			}
		}
		// Move to next field
		m.createFocus = (m.createFocus + 1) % 3
		// Focus new field
		if m.createFocus == 0 {
			m.createInputs[0].Focus()
			return m, textinput.Blink
		} else if m.createFocus == 1 {
			if m.createEditorMode {
				m.createValueArea.Focus()
				return m, nil
			}
			m.createInputs[1].Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "shift+tab":
		// Blur current field
		if m.createFocus == 0 {
			m.createInputs[0].Blur()
		} else if m.createFocus == 1 {
			if m.createEditorMode {
				m.createValueArea.Blur()
			} else {
				m.createInputs[1].Blur()
			}
		}
		// Move to previous field
		m.createFocus--
		if m.createFocus < 0 {
			m.createFocus = 2
		}
		// Focus new field
		if m.createFocus == 0 {
			m.createInputs[0].Focus()
			return m, textinput.Blink
		} else if m.createFocus == 1 {
			if m.createEditorMode {
				m.createValueArea.Focus()
				return m, nil
			}
			m.createInputs[1].Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "enter":
		// In editor mode, enter adds newline - use Ctrl+Enter to submit
		if m.createEditorMode && m.createFocus == 1 {
			// Let textarea handle enter for newlines
			var cmd tea.Cmd
			m.createValueArea, cmd = m.createValueArea.Update(msg)
			return m, cmd
		}
		// If on location field and "Add new" is selected
		if m.createFocus == 2 && m.createLocationIdx == -1 {
			m.createAddingLoc = true
			m.createLocInput.Focus()
			return m, textinput.Blink
		}
		// Submit form
		name := m.createInputs[0].Value()
		var value string
		if m.createEditorMode {
			value = m.createValueArea.Value()
		} else {
			value = m.createInputs[1].Value()
		}
		if name == "" {
			m.statusMsg = "Secret name is required"
			m.statusErr = true
			return m, nil
		}
		// Get selected location: 0=global (empty), 1+=config.SecretLocations[idx-1]
		var location string
		if m.createLocationIdx > 0 && m.createLocationIdx <= len(m.config.SecretLocations) {
			location = m.config.SecretLocations[m.createLocationIdx-1]
		}
		// location stays empty for global (index 0)
		m.loading = true
		m.loadingMsg = "Creating secret..."
		// Clear inputs
		m.createInputs[0].SetValue("")
		m.createInputs[1].SetValue("")
		m.createValueArea.SetValue("")
		m.createLocationIdx = 0
		m.createEditorMode = false
		return m, m.createSecret(name, []byte(value), location)
	case "ctrl+s":
		// Alternative submit shortcut (useful in editor mode)
		name := m.createInputs[0].Value()
		var value string
		if m.createEditorMode {
			value = m.createValueArea.Value()
		} else {
			value = m.createInputs[1].Value()
		}
		if name == "" {
			m.statusMsg = "Secret name is required"
			m.statusErr = true
			return m, nil
		}
		var location string
		if m.createLocationIdx > 0 && m.createLocationIdx <= len(m.config.SecretLocations) {
			location = m.config.SecretLocations[m.createLocationIdx-1]
		}
		m.loading = true
		m.loadingMsg = "Creating secret..."
		m.createInputs[0].SetValue("")
		m.createInputs[1].SetValue("")
		m.createValueArea.SetValue("")
		m.createLocationIdx = 0
		m.createEditorMode = false
		return m, m.createSecret(name, []byte(value), location)
	case "esc":
		m.view = ViewList
		m.createInputs[0].SetValue("")
		m.createInputs[1].SetValue("")
		m.createValueArea.SetValue("")
		m.createLocationIdx = 0
		m.createAddingLoc = false
		m.createEditorMode = false
		return m, nil
	case "left", "h":
		// Navigate location options (only when on location field)
		if m.createFocus == 2 {
			m.createLocationIdx--
			// -1 = add new, 0 = global, 1..n = regions
			if m.createLocationIdx < -1 {
				m.createLocationIdx = len(m.config.SecretLocations)
			}
			return m, nil
		}
	case "right", "l":
		// Navigate location options (only when on location field)
		if m.createFocus == 2 {
			m.createLocationIdx++
			// -1 = add new, 0 = global, 1..n = regions
			if m.createLocationIdx > len(m.config.SecretLocations) {
				m.createLocationIdx = -1
			}
			return m, nil
		}
	}
	
	// Update focused field
	if m.createFocus == 0 {
		var cmd tea.Cmd
		m.createInputs[0], cmd = m.createInputs[0].Update(msg)
		return m, cmd
	} else if m.createFocus == 1 {
		if m.createEditorMode {
			var cmd tea.Cmd
			m.createValueArea, cmd = m.createValueArea.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.createInputs[1], cmd = m.createInputs[1].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateAddVersion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := m.versionInput.Value()
		if value == "" {
			m.statusMsg = "Value is required"
			m.statusErr = true
			return m, nil
		}
		m.loading = true
		m.loadingMsg = "Adding version..."
		m.versionInput.SetValue("")
		return m, m.addVersion(m.selectedSecret.Name, []byte(value))
	case "esc":
		m.view = ViewDetail
		m.versionInput.SetValue("")
		return m, nil
	}
	
	var cmd tea.Cmd
	m.versionInput, cmd = m.versionInput.Update(msg)
	return m, cmd
}

func (m Model) updateDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.loading = true
		m.loadingMsg = "Deleting secret..."
		return m, m.deleteSecret(m.selectedSecret.Name)
	case "n", "esc":
		if m.previousView == ViewDetail {
			m.view = ViewDetail
		} else {
			m.view = ViewList
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateGenerate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.templateCursor > 0 {
			m.templateCursor--
		}
	case "down", "j":
		if m.templateCursor < len(m.config.Templates)-1 {
			m.templateCursor++
		}
	case "enter":
		m.generatedCode = m.generateCode(m.templateCursor)
	case "c", "y":
		// Copy generated code to clipboard
		if m.generatedCode != "" {
			err := clipboard.WriteText(m.generatedCode)
			if err != nil {
				m.statusMsg = "Failed to copy to clipboard"
				m.statusErr = true
			} else {
				m.statusMsg = "Code copied to clipboard!"
				m.statusErr = false
				// Set up clipboard auto-clear if enabled
				if m.config.Clipboard.AutoClear {
					m.clipboardClearAt = time.Now().Add(time.Duration(m.config.Clipboard.TimeoutSeconds) * time.Second)
					m.clipboardActive = true
					return m, clipboardTickCmd()
				}
			}
		}
	case "esc", "backspace", "h":
		m.view = ViewDetail
		m.generatedCode = ""
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfigMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.configMenuCursor > 0 {
			m.configMenuCursor--
		}
	case "down", "j":
		if m.configMenuCursor < len(m.configMenuItems)-1 {
			m.configMenuCursor++
		}
	case "enter", "l":
		switch m.configMenuCursor {
		case 0: // Basic Settings
			m.view = ViewConfigBasic
			m.configInputs[0].SetValue(m.config.ProjectID)
			m.configInputs[1].SetValue(m.config.FolderSeparator)
			m.configInputs[0].Focus()
			m.configFocus = 0
			return m, textinput.Blink
		case 1: // Templates
			m.view = ViewConfigTemplates
			m.templateListCursor = 0
		case 2: // Recent Projects
			m.view = ViewConfigRecentProjects
			m.recentProjectsCursor = 0
		case 3: // Security Settings
			m.view = ViewConfigSecurity
			m.securityCursor = 0
		case 4: // Save & Exit
			_ = m.config.Save()
			m.statusMsg = "Configuration saved"
			m.statusErr = false
			m.view = ViewList
			m.buildFolderTree()
			m.updateDisplayItems()
		}
	case "esc", "q":
		m.view = ViewList
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfigBasic(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.configInputs[m.configFocus].Blur()
		m.configFocus = (m.configFocus + 1) % len(m.configInputs)
		m.configInputs[m.configFocus].Focus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.configInputs[m.configFocus].Blur()
		m.configFocus--
		if m.configFocus < 0 {
			m.configFocus = len(m.configInputs) - 1
		}
		m.configInputs[m.configFocus].Focus()
		return m, textinput.Blink
	case "enter":
		m.config.ProjectID = m.configInputs[0].Value()
		sep := m.configInputs[1].Value()
		if sep != "" {
			m.config.FolderSeparator = sep
		}
		m.statusMsg = "Settings updated"
		m.statusErr = false
		m.view = ViewConfigMenu
		return m, nil
	case "esc":
		m.view = ViewConfigMenu
		return m, nil
	}
	
	var cmd tea.Cmd
	m.configInputs[m.configFocus], cmd = m.configInputs[m.configFocus].Update(msg)
	return m, cmd
}

func (m Model) updateConfigTemplates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.templateListCursor > 0 {
			m.templateListCursor--
		}
	case "down", "j":
		if m.templateListCursor < len(m.config.Templates) {
			m.templateListCursor++
		}
	case "enter", "e":
		if m.templateListCursor < len(m.config.Templates) {
			// Edit existing template
			m.editingTemplateIdx = m.templateListCursor
			m.isNewTemplate = false
			tpl := m.config.Templates[m.templateListCursor]
			m.templateTitleInput.SetValue(tpl.Title)
			m.templateCodeArea.SetValue(tpl.Code)
			m.templateTitleInput.Focus()
			m.templateCodeArea.Blur()
			m.templateFocus = 0
			m.view = ViewConfigTemplateEdit
			return m, textinput.Blink
		}
	case "n":
		// New template
		m.isNewTemplate = true
		m.editingTemplateIdx = -1
		m.templateTitleInput.SetValue("")
		m.templateCodeArea.SetValue("")
		m.templateTitleInput.Focus()
		m.templateCodeArea.Blur()
		m.templateFocus = 0
		m.view = ViewConfigTemplateEdit
		return m, textinput.Blink
	case "d":
		if m.templateListCursor < len(m.config.Templates) && len(m.config.Templates) > 1 {
			// Delete template
			m.config.Templates = append(
				m.config.Templates[:m.templateListCursor],
				m.config.Templates[m.templateListCursor+1:]...,
			)
			if m.templateListCursor >= len(m.config.Templates) {
				m.templateListCursor = len(m.config.Templates) - 1
			}
			m.statusMsg = "Template deleted"
			m.statusErr = false
		}
	case "esc", "backspace", "h":
		m.view = ViewConfigMenu
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfigTemplateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.templateFocus == 0 {
			m.templateTitleInput.Blur()
			m.templateCodeArea.Focus()
			m.templateFocus = 1
			return m, textarea.Blink
		} else {
			m.templateCodeArea.Blur()
			m.templateTitleInput.Focus()
			m.templateFocus = 0
			return m, textinput.Blink
		}
	case "ctrl+s":
		title := m.templateTitleInput.Value()
		code := m.templateCodeArea.Value()
		if title == "" {
			m.statusMsg = "Template title is required"
			m.statusErr = true
			return m, nil
		}
		
		newTpl := config.Template{Title: title, Code: code}
		
		if m.isNewTemplate {
			m.config.Templates = append(m.config.Templates, newTpl)
			m.statusMsg = "Template created"
		} else {
			m.config.Templates[m.editingTemplateIdx] = newTpl
			m.statusMsg = "Template updated"
		}
		m.statusErr = false
		m.view = ViewConfigTemplates
		return m, nil
	case "esc":
		// Only escape if not in textarea or if textarea is empty
		if m.templateFocus == 0 || m.templateCodeArea.Value() == "" {
			m.view = ViewConfigTemplates
			return m, nil
		}
	}
	
	var cmd tea.Cmd
	if m.templateFocus == 0 {
		m.templateTitleInput, cmd = m.templateTitleInput.Update(msg)
	} else {
		m.templateCodeArea, cmd = m.templateCodeArea.Update(msg)
	}
	return m, cmd
}

func (m Model) updateConfigRecentProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.recentProjectsCursor > 0 {
			m.recentProjectsCursor--
		}
	case "down", "j":
		if m.recentProjectsCursor < len(m.config.RecentProjects)-1 {
			m.recentProjectsCursor++
		}
	case "enter":
		if len(m.config.RecentProjects) > 0 && m.recentProjectsCursor < len(m.config.RecentProjects) {
			// Switch to this project
			m.config.ProjectID = m.config.RecentProjects[m.recentProjectsCursor]
			m.statusMsg = fmt.Sprintf("Switched to project: %s", m.config.ProjectID)
			m.statusErr = false
			_ = m.config.Save()
			m.view = ViewList
			m.loading = true
			m.loadingMsg = "Connecting to GCP..."
			return m, m.initializeClient()
		}
	case "d":
		if len(m.config.RecentProjects) > 0 && m.recentProjectsCursor < len(m.config.RecentProjects) {
			// Remove from recent
			m.config.RecentProjects = append(
				m.config.RecentProjects[:m.recentProjectsCursor],
				m.config.RecentProjects[m.recentProjectsCursor+1:]...,
			)
			if m.recentProjectsCursor >= len(m.config.RecentProjects) && m.recentProjectsCursor > 0 {
				m.recentProjectsCursor--
			}
			m.statusMsg = "Project removed from history"
			m.statusErr = false
		}
	case "esc", "backspace", "h":
		m.view = ViewConfigMenu
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfigSecurity(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Security options: 0 = Auto-clear, 1 = Timeout, 2 = Audit enabled, 3 = Retention, 4 = View logs
	// 5 = Lock on timeout, 6 = Inactivity timeout
	maxOptions := 7
	
	switch msg.String() {
	case "up", "k":
		if m.securityCursor > 0 {
			m.securityCursor--
		}
	case "down", "j":
		if m.securityCursor < maxOptions-1 {
			m.securityCursor++
		}
	case "enter", " ":
		switch m.securityCursor {
		case 0: // Toggle auto-clear
			m.config.Clipboard.AutoClear = !m.config.Clipboard.AutoClear
			if m.config.Clipboard.AutoClear {
				m.statusMsg = "✓ Clipboard auto-clear enabled"
			} else {
				m.statusMsg = "○ Clipboard auto-clear disabled"
			}
			m.statusErr = false
		case 1: // Cycle clipboard timeout (15, 30, 60, 120 seconds)
			timeouts := []int{15, 30, 60, 120}
			currentIdx := 0
			for i, t := range timeouts {
				if t == m.config.Clipboard.TimeoutSeconds {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(timeouts)
			m.config.Clipboard.TimeoutSeconds = timeouts[nextIdx]
			m.statusMsg = fmt.Sprintf("Clipboard timeout: %d seconds", m.config.Clipboard.TimeoutSeconds)
			m.statusErr = false
		case 2: // Toggle audit logging
			oldEnabled := m.config.Audit.Enabled
			m.config.Audit.Enabled = !m.config.Audit.Enabled
			if m.config.Audit.Enabled {
				m.statusMsg = "✓ Audit logging enabled"
				// Reinitialize audit logger
				auditCfg := audit.Config{
					Enabled:    true,
					FilePath:   m.config.Audit.FilePath,
					MaxSizeMB:  m.config.Audit.MaxSizeMB,
					MaxAgeDays: m.config.Audit.MaxAgeDays,
				}
				if newLogger, err := audit.NewLogger(auditCfg); err == nil {
					if m.auditLogger != nil {
						m.auditLogger.Close()
					}
					m.auditLogger = newLogger
				}
			} else {
				m.statusMsg = "○ Audit logging disabled"
				if m.auditLogger != nil {
					m.auditLogger.Close()
					m.auditLogger = nil
				}
			}
			if m.auditLogger != nil && oldEnabled != m.config.Audit.Enabled {
				m.auditLogger.LogConfigChange("audit.enabled", fmt.Sprintf("%v", oldEnabled), fmt.Sprintf("%v", m.config.Audit.Enabled))
			}
			m.statusErr = false
		case 3: // Cycle audit retention (30, 60, 90, 180, 365 days)
			retentions := []int{30, 60, 90, 180, 365}
			currentIdx := 0
			for i, r := range retentions {
				if r == m.config.Audit.MaxAgeDays {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(retentions)
			m.config.Audit.MaxAgeDays = retentions[nextIdx]
			m.statusMsg = fmt.Sprintf("Audit retention: %d days", m.config.Audit.MaxAgeDays)
			m.statusErr = false
		case 4: // View audit logs
			m.loadAuditLogs()
			m.view = ViewAuditLog
			return m, nil
		case 5: // Toggle lock on timeout
			m.config.Session.LockOnTimeout = !m.config.Session.LockOnTimeout
			if m.config.Session.LockOnTimeout {
				m.statusMsg = "✓ Session lock on timeout enabled"
			} else {
				m.statusMsg = "○ Session lock on timeout disabled"
			}
			m.statusErr = false
		case 6: // Cycle inactivity timeout (0=disabled, 5, 10, 15, 30, 60 minutes)
			timeouts := []int{0, 5, 10, 15, 30, 60}
			currentIdx := 0
			for i, t := range timeouts {
				if t == m.config.Session.InactivityTimeout {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(timeouts)
			m.config.Session.InactivityTimeout = timeouts[nextIdx]
			if m.config.Session.InactivityTimeout == 0 {
				m.statusMsg = "Inactivity timeout: Disabled"
			} else {
				m.statusMsg = fmt.Sprintf("Inactivity timeout: %d minutes", m.config.Session.InactivityTimeout)
			}
			m.statusErr = false
		}
	case "esc", "backspace", "h":
		m.view = ViewConfigMenu
		return m, nil
	}
	return m, nil
}

func (m *Model) loadAuditLogs() {
	if m.auditLogger != nil {
		lines, err := m.auditLogger.ReadRecentLogs(500)
		if err == nil {
			m.auditLogLines = lines
		} else {
			m.auditLogLines = []string{"Error reading logs: " + err.Error()}
		}
	} else {
		m.auditLogLines = []string{"Audit logging is disabled"}
	}
	m.auditLogOffset = 0
}

func (m Model) updateAuditLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}
	maxOffset := len(m.auditLogLines) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}

	switch msg.String() {
	case "up", "k":
		if m.auditLogOffset > 0 {
			m.auditLogOffset--
		}
	case "down", "j":
		if m.auditLogOffset < maxOffset {
			m.auditLogOffset++
		}
	case "g":
		m.auditLogOffset = 0
	case "G":
		m.auditLogOffset = maxOffset
	case "r":
		m.loadAuditLogs()
		m.statusMsg = "Logs refreshed"
		m.statusErr = false
	case "esc", "backspace", "h":
		m.view = ViewConfigSecurity
		return m, nil
	}
	return m, nil
}

func (m Model) updateLocked(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		// Unlock session
		m.sessionLocked = false
		m.view = m.lockedPrevView
		m.lastActivity = time.Now()
		if m.auditLogger != nil {
			m.auditLogger.LogSessionUnlock(m.config.ProjectID)
		}
		m.statusMsg = "🔓 Session unlocked"
		m.statusErr = false
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateProjectSwitch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter recent projects based on input
	filterText := m.projectSwitchInput.Value()
	filteredProjects := m.getFilteredProjects(filterText)
	
	// Calculate max visible items
	maxShow := 15
	if len(filteredProjects) < maxShow {
		maxShow = len(filteredProjects)
	}
	
	switch msg.String() {
	case "up", "ctrl+k":
		if m.projectSwitchCursor > 0 {
			m.projectSwitchCursor--
		}
	case "down", "ctrl+j":
		maxIdx := maxShow
		if filterText != "" {
			maxIdx++ // Include "use typed value" option
		}
		if m.projectSwitchCursor < maxIdx-1 {
			m.projectSwitchCursor++
		}
	case "enter":
		var selectedProject string
		
		if filterText != "" && m.projectSwitchCursor == 0 {
			// Use the typed value
			selectedProject = filterText
		} else {
			// Select from filtered list
			idx := m.projectSwitchCursor
			if filterText != "" {
				idx-- // Adjust for "use typed value" option
			}
			if idx >= 0 && idx < len(filteredProjects) {
				selectedProject = filteredProjects[idx]
			}
		}
		
		if selectedProject != "" && selectedProject != m.config.ProjectID {
			oldProject := m.config.ProjectID
			m.config.ProjectID = selectedProject
			m.config.AddRecentProject(selectedProject)
			_ = m.config.Save()
			if m.auditLogger != nil {
				m.auditLogger.LogProjectSwitch(oldProject, selectedProject)
			}
			m.statusMsg = fmt.Sprintf("Switched to: %s", selectedProject)
			m.statusErr = false
			m.view = ViewList
			m.loading = true
			m.loadingMsg = "Connecting to GCP..."
			m.currentPath = []string{}
			m.cursor = 0
			return m, m.initializeClient()
		} else if selectedProject == m.config.ProjectID {
			m.statusMsg = "Already on this project"
			m.statusErr = false
			m.view = m.projectSwitchPrevView
		}
		return m, nil
	case "esc":
		m.view = m.projectSwitchPrevView
		return m, nil
	}
	
	// Update input and reset cursor if text changed
	var cmd tea.Cmd
	oldValue := m.projectSwitchInput.Value()
	m.projectSwitchInput, cmd = m.projectSwitchInput.Update(msg)
	if m.projectSwitchInput.Value() != oldValue {
		m.projectSwitchCursor = 0
	}
	
	return m, cmd
}

// getFilteredProjects returns projects matching the filter
func (m Model) getFilteredProjects(filter string) []string {
	if filter == "" {
		return m.config.RecentProjects
	}
	
	var filtered []string
	filterLower := strings.ToLower(filter)
	for _, p := range m.config.RecentProjects {
		if strings.Contains(strings.ToLower(p), filterLower) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.view = ViewList
		m.filterText = m.filterInput.Value()
		m.updateDisplayItems()
		m.cursor = 0
		return m, nil
	}
	
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterText = m.filterInput.Value()
	m.updateDisplayItems()
	return m, cmd
}

func (m Model) updateReveal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace", "enter", "r":
		m.view = ViewDetail
		// Securely clear secret from memory
		for i := range m.revealedValue {
			m.revealedValue[i] = 0
		}
		m.revealedValue = nil
		m.revealVersion = ""
		return m, nil
	case "c", "y":
		// Copy revealed value to clipboard
		if len(m.revealedValue) > 0 {
			err := clipboard.WriteText(string(m.revealedValue))
			if err != nil {
				m.statusMsg = fmt.Sprintf("Error copying: %v", err)
				m.statusErr = true
				return m, nil
			}
			// Start clipboard auto-clear timer if enabled
			if m.config.Clipboard.AutoClear && m.config.Clipboard.TimeoutSeconds > 0 {
				timeout := time.Duration(m.config.Clipboard.TimeoutSeconds) * time.Second
				m.clipboardClearAt = time.Now().Add(timeout)
				m.clipboardActive = true
				remaining := m.config.Clipboard.TimeoutSeconds
				m.statusMsg = fmt.Sprintf("📋 Copied! Auto-clear in %ds", remaining)
				m.statusErr = false
				return m, clipboardTickCmd()
			}
			m.statusMsg = "✓ Secret value copied to clipboard"
			m.statusErr = false
		}
		return m, nil
	}
	return m, nil
}

// buildFolderTree builds the folder tree from secrets
func (m *Model) buildFolderTree() {
	m.folderTree = &FolderItem{
		Name:     "",
		Children: make(map[string]*FolderItem),
	}
	
	for i := range m.secrets {
		secret := &m.secrets[i]
		parts := strings.Split(secret.Name, m.config.FolderSeparator)
		
		current := m.folderTree
		for j, part := range parts {
			if part == "" {
				continue
			}
			
			if _, exists := current.Children[part]; !exists {
				isFolder := j < len(parts)-1
				current.Children[part] = &FolderItem{
					Name:     part,
					FullPath: strings.Join(parts[:j+1], m.config.FolderSeparator),
					IsFolder: isFolder,
					Children: make(map[string]*FolderItem),
					Depth:    j,
				}
				if !isFolder {
					current.Children[part].Secret = secret
				}
			}
			current = current.Children[part]
		}
	}
}

// updateDisplayItems updates the list of items to display based on current path and filter
func (m *Model) updateDisplayItems() {
	m.displayItems = []*FolderItem{}
	
	// Navigate to current path
	current := m.folderTree
	for _, p := range m.currentPath {
		if child, exists := current.Children[p]; exists {
			current = child
		} else {
			return
		}
	}
	
	// Collect items
	items := make([]*FolderItem, 0, len(current.Children))
	for _, item := range current.Children {
		// Apply filter
		if m.filterText != "" {
			if !strings.Contains(strings.ToLower(item.Name), strings.ToLower(m.filterText)) {
				continue
			}
		}
		items = append(items, item)
	}
	
	// Sort: folders first, then by name
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsFolder != items[j].IsFolder {
			return items[i].IsFolder
		}
		return items[i].Name < items[j].Name
	})
	
	m.displayItems = items
	
	// Adjust cursor if needed
	if m.cursor >= len(m.displayItems) {
		m.cursor = len(m.displayItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	
	// Reset scroll offset
	m.listOffset = 0
}

// generateCode generates code from a template
func (m Model) generateCode(templateIdx int) string {
	if templateIdx >= len(m.config.Templates) || m.selectedSecret == nil {
		return ""
	}
	
	tpl := m.config.Templates[templateIdx]
	
	// Extract just the secret name (last part)
	parts := strings.Split(m.selectedSecret.Name, m.config.FolderSeparator)
	shortName := parts[len(parts)-1]
	
	data := map[string]string{
		"SecretName":     shortName,
		"FullSecretName": m.selectedSecret.Name,
		"ProjectID":      m.config.ProjectID,
	}
	
	t, err := template.New("code").Parse(tpl.Code)
	if err != nil {
		return fmt.Sprintf("Template error: %v", err)
	}
	
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Execution error: %v", err)
	}
	
	return buf.String()
}

// View renders the UI
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	
	var content string
	var footer []FooterBinding
	
	switch m.view {
	case ViewProjectPrompt:
		content = m.viewProjectPrompt()
		footer = InputViewBindings()
	case ViewList:
		content = m.viewList()
		footer = ListViewBindings()
	case ViewDetail:
		content = m.viewDetail()
		footer = DetailViewBindings()
	case ViewCreate:
		content = m.viewCreate()
		footer = CreateViewBindings()
	case ViewAddVersion:
		content = m.viewAddVersion()
		footer = InputViewBindings()
	case ViewDelete:
		content = m.viewDelete()
		footer = ConfirmViewBindings()
	case ViewGenerate:
		content = m.viewGenerate()
		footer = GenerateViewBindings()
	case ViewConfig, ViewConfigMenu:
		content = m.viewConfigMenu()
		footer = ConfigMenuBindings()
	case ViewConfigBasic:
		content = m.viewConfigBasic()
		footer = InputViewBindings()
	case ViewConfigTemplates:
		content = m.viewConfigTemplates()
		footer = ConfigTemplatesBindings()
	case ViewConfigTemplateEdit:
		content = m.viewConfigTemplateEdit()
		footer = ConfigTemplateEditBindings()
	case ViewConfigRecentProjects:
		content = m.viewConfigRecentProjects()
		footer = ConfigRecentBindings()
	case ViewConfigSecurity:
		content = m.viewConfigSecurity()
		footer = ConfigSecurityBindings()
	case ViewAuditLog:
		content = m.viewAuditLog()
		footer = AuditLogBindings()
	case ViewFilter:
		content = m.viewFilter()
		footer = InputViewBindings()
	case ViewReveal:
		content = m.viewReveal()
		footer = RevealViewBindings()
	case ViewProjectSwitch:
		content = m.viewProjectSwitch()
		footer = ProjectSwitchBindings()
	case ViewLocked:
		content = m.viewLocked()
		footer = LockedViewBindings()
	}
	
	return m.renderLayout(content, footer)
}

func (m Model) renderLayout(content string, footerBindings []FooterBinding) string {
	// Header
	header := m.styles.Header.Width(m.width).Render(
		fmt.Sprintf("🔐 GCP Secret Manager  │  %s", m.config.ProjectID),
	)
	
	// Footer with keybindings
	var footerParts []string
	for _, b := range footerBindings {
		key := m.styles.FooterKey.Render(b.Key)
		desc := m.styles.FooterDesc.Render(b.Desc)
		footerParts = append(footerParts, key+desc)
	}
	footerContent := lipgloss.JoinHorizontal(lipgloss.Left, footerParts...)
	footer := m.styles.Footer.Width(m.width).Render(footerContent)
	
	// Status line
	statusStyle := m.styles.StatusInfo
	if m.statusErr {
		statusStyle = m.styles.StatusError
	}
	status := statusStyle.Render(m.statusMsg)
	
	// Calculate content height
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	statusHeight := 1
	contentHeight := m.height - headerHeight - footerHeight - statusHeight - 1
	
	// Content area
	contentArea := m.styles.Content.
		Width(m.width - 4).
		Height(contentHeight).
		Render(content)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		contentArea,
		status,
		footer,
	)
}

func (m Model) viewProjectPrompt() string {
	if m.loading {
		return m.viewSplash(m.loadingMsg, "⏳", "GCP Secret Manager TUI")
	}
	
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("Welcome to GCP Secret Manager"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.InputLabel.Render("Project ID:"))
	b.WriteString("\n")
	b.WriteString(m.styles.InputFocused.Render(m.configInputs[0].View()))
	b.WriteString("\n\n")
	
	if len(m.config.RecentProjects) > 0 {
		b.WriteString(m.styles.DetailLabel.Render("Recent projects:"))
		b.WriteString("\n")
		for _, p := range m.config.RecentProjects {
			b.WriteString(m.styles.ListItem.Render("  • " + p))
			b.WriteString("\n")
		}
	}
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewList() string {
	if m.loading {
		return m.viewSplash(m.loadingMsg, "⏳", "")
	}
	
	var b strings.Builder
	
	// Breadcrumb
	breadcrumb := m.renderBreadcrumb()
	b.WriteString(breadcrumb)
	b.WriteString("\n\n")
	
	// Filter indicator
	if m.filterText != "" {
		b.WriteString(m.styles.StatusWarning.Render(fmt.Sprintf("Filter: %s", m.filterText)))
		b.WriteString("\n\n")
	}
	
	// List items
	if len(m.displayItems) == 0 {
		b.WriteString(m.styles.SubtleText().Render("No secrets found"))
	} else {
		// Calculate visible window
		visibleHeight := m.height - 10 // Leave room for header, footer, breadcrumb, etc.
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		
		startIdx := m.listOffset
		endIdx := startIdx + visibleHeight
		if endIdx > len(m.displayItems) {
			endIdx = len(m.displayItems)
		}
		
		// Show scroll indicator at top if needed
		if startIdx > 0 {
			b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("  ↑ %d more items above", startIdx)))
			b.WriteString("\n")
		}
		
		// Render visible items
		for i := startIdx; i < endIdx; i++ {
			item := m.displayItems[i]
			var line string
			icon := "🔑"
			nameStyle := m.styles.ListSecret
			if item.IsFolder {
				icon = "📁"
				nameStyle = m.styles.ListFolder
			}
			
			name := nameStyle.Render(item.Name)
			line = fmt.Sprintf("%s %s", icon, name)
			
			if i == m.cursor {
				line = m.styles.ListSelected.Width(m.width - 6).Render(line)
			} else {
				line = m.styles.ListItem.Width(m.width - 6).Render(line)
			}
			
			b.WriteString(line)
			b.WriteString("\n")
		}
		
		// Show scroll indicator at bottom if needed
		remaining := len(m.displayItems) - endIdx
		if remaining > 0 {
			b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("  ↓ %d more items below", remaining)))
			b.WriteString("\n")
		}
		
		// Show position indicator
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("  %d/%d", m.cursor+1, len(m.displayItems))))
	}
	
	return b.String()
}

func (m Model) viewDetail() string {
	if m.loading {
		return m.viewSplash(m.loadingMsg, "⏳", "")
	}
	
	if m.selectedSecret == nil {
		return "No secret selected"
	}
	
	var b strings.Builder
	
	// Title
	b.WriteString(m.styles.DetailTitle.Width(m.width - 6).Render(
		fmt.Sprintf("🔐 %s", m.selectedSecret.Name),
	))
	b.WriteString("\n\n")
	
	// Details
	b.WriteString(m.styles.DetailLabel.Render("Created:"))
	b.WriteString(m.styles.DetailValue.Render(m.selectedSecret.CreateTime))
	b.WriteString("\n")
	
	b.WriteString(m.styles.DetailLabel.Render("Replication:"))
	b.WriteString(m.styles.DetailValue.Render(m.selectedSecret.Replication))
	b.WriteString("\n\n")
	
	// Versions
	b.WriteString(m.styles.ListTitle.Render("Versions"))
	b.WriteString("\n")
	
	if len(m.versions) == 0 {
		b.WriteString(m.styles.SubtleText().Render("  No versions"))
	} else {
		for i, v := range m.versions {
			stateIcon := "✓"
			stateStyle := m.styles.StatusSuccess
			if v.State == "DISABLED" {
				stateIcon = "○"
				stateStyle = m.styles.StatusWarning
			} else if v.State == "DESTROYED" {
				stateIcon = "✕"
				stateStyle = m.styles.StatusError
			}
			
			line := fmt.Sprintf("%s v%s  %s  %s",
				stateStyle.Render(stateIcon),
				m.styles.DetailVersion.Render(v.Name),
				m.styles.SubtleText().Render(v.CreateTime),
				m.styles.SubtleText().Render(v.State),
			)
			
			if i == m.versionCursor {
				line = m.styles.ListSelected.Width(m.width - 6).Render(line)
			} else {
				line = m.styles.ListItem.Width(m.width - 6).Render(line)
			}
			
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	
	return b.String()
}

func (m Model) viewCreate() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("Create New Secret"))
	b.WriteString("\n\n")
	
	// Name input
	b.WriteString(m.styles.InputLabel.Render("Secret Name:"))
	b.WriteString("\n")
	inputStyle := m.styles.Input
	if m.createFocus == 0 {
		inputStyle = m.styles.InputFocused
	}
	b.WriteString(inputStyle.Width(50).Render(m.createInputs[0].View()))
	b.WriteString("\n\n")
	
	// Value input with editor mode toggle
	valueLabel := "Secret Value:"
	if m.createEditorMode {
		valueLabel = "Secret Value (Editor Mode):"
	}
	b.WriteString(m.styles.InputLabel.Render(valueLabel))
	b.WriteString("  ")
	// Toggle indicator
	if m.createEditorMode {
		b.WriteString(m.styles.StatusSuccess.Render("[Editor]"))
	} else {
		b.WriteString(m.styles.SubtleText().Render("[Password]"))
	}
	b.WriteString(m.styles.SubtleText().Render("  Ctrl+E to toggle"))
	b.WriteString("\n")
	
	if m.createEditorMode {
		// Show textarea for multiline input
		areaStyle := m.styles.Input
		if m.createFocus == 1 {
			areaStyle = m.styles.InputFocused
		}
		b.WriteString(areaStyle.Render(m.createValueArea.View()))
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render("  Ctrl+S to submit • Supports multiline (PEM keys, JSON, etc.)"))
	} else {
		// Show password input
		inputStyle = m.styles.Input
		if m.createFocus == 1 {
			inputStyle = m.styles.InputFocused
		}
		b.WriteString(inputStyle.Width(50).Render(m.createInputs[1].View()))
	}
	b.WriteString("\n\n")
	
	// Location selector
	b.WriteString(m.styles.InputLabel.Render("Location:"))
	if m.createFocus == 2 {
		b.WriteString(m.styles.FooterKey.Render(" (←/→ to navigate, Enter to select)"))
	}
	b.WriteString("\n")
	
	// Show adding new location input
	if m.createAddingLoc {
		b.WriteString(m.styles.InputFocused.Width(50).Render(m.createLocInput.View()))
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render("Enter location (e.g. europe-west1) and press Enter"))
	} else {
		// Build location options: global first, then saved locations, then add new
		var locOptions []string
		
		// Option 0: Global (automatic replication)
		globalOption := "🌐 global (automatic replication)"
		if m.createLocationIdx == 0 {
			if m.createFocus == 2 {
				globalOption = m.styles.ListSelected.Render("▶ " + globalOption)
			} else {
				globalOption = m.styles.FooterKey.Render("● " + globalOption)
			}
		} else {
			globalOption = m.styles.SubtleText().Render("  " + globalOption)
		}
		locOptions = append(locOptions, globalOption)
		
		// Options 1..n: Saved locations
		for i, loc := range m.config.SecretLocations {
			option := "📍 " + loc
			if m.createLocationIdx == i+1 { // +1 because 0 is global
				if m.createFocus == 2 {
					option = m.styles.ListSelected.Render("▶ " + option)
				} else {
					option = m.styles.FooterKey.Render("● " + option)
				}
			} else {
				option = m.styles.SubtleText().Render("  " + option)
			}
			locOptions = append(locOptions, option)
		}
		
		// Option -1: Add new location
		addNewOption := "➕ Add new location..."
		if m.createLocationIdx == -1 {
			if m.createFocus == 2 {
				addNewOption = m.styles.ListSelected.Render("▶ " + addNewOption)
			} else {
				addNewOption = m.styles.FooterKey.Render("● " + addNewOption)
			}
		} else {
			addNewOption = m.styles.SubtleText().Render("  " + addNewOption)
		}
		locOptions = append(locOptions, addNewOption)
		
		// Render options
		for _, opt := range locOptions {
			b.WriteString(opt)
			b.WriteString("\n")
		}
	}
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewAddVersion() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render(
		fmt.Sprintf("Add Version to %s", m.selectedSecret.Name),
	))
	b.WriteString("\n\n")
	
	b.WriteString(m.styles.InputLabel.Render("New Value:"))
	b.WriteString("\n")
	b.WriteString(m.styles.InputFocused.Width(50).Render(m.versionInput.View()))
	b.WriteString("\n")
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewDelete() string {
	var b strings.Builder
	
	b.WriteString(m.styles.StatusError.Bold(true).Render("⚠ Delete Secret"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Are you sure you want to delete '%s'?\n", m.selectedSecret.Name))
	b.WriteString(m.styles.SubtleText().Render("This action cannot be undone. All versions will be destroyed."))
	b.WriteString("\n\n")
	b.WriteString("Press ")
	b.WriteString(m.styles.FooterKey.Render("y"))
	b.WriteString(" to confirm or ")
	b.WriteString(m.styles.FooterKey.Render("n"))
	b.WriteString(" to cancel")
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewGenerate() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("Generate Code"))
	b.WriteString("\n\n")
	
	b.WriteString(m.styles.InputLabel.Render("Select Template:"))
	b.WriteString("\n\n")
	
	for i, tpl := range m.config.Templates {
		line := tpl.Title
		if i == m.templateCursor {
			line = m.styles.ListSelected.Width(40).Render("▶ " + line)
		} else {
			line = m.styles.ListItem.Width(40).Render("  " + line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	
	if m.generatedCode != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.InputLabel.Render("Generated Code:"))
		b.WriteString("  ")
		b.WriteString(m.styles.SubtleText().Render("(press c to copy)"))
		b.WriteString("\n")
		b.WriteString(m.styles.CodeBlock.Render(m.generatedCode))
	}
	
	return b.String()
}

func (m Model) viewConfigMenu() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("⚙ Settings"))
	b.WriteString("\n\n")
	
	for i, item := range m.configMenuItems {
		line := item
		if i == m.configMenuCursor {
			line = m.styles.ListSelected.Width(40).Render("▶ " + line)
		} else {
			line = m.styles.ListItem.Width(40).Render("  " + line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Current project: " + m.config.ProjectID))
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Folder separator: \"" + m.config.FolderSeparator + "\""))
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("Templates: %d", len(m.config.Templates))))
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewConfigBasic() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("📋 Basic Settings"))
	b.WriteString("\n\n")
	
	// Project ID
	b.WriteString(m.styles.InputLabel.Render("Project ID:"))
	b.WriteString("\n")
	inputStyle := m.styles.Input
	if m.configFocus == 0 {
		inputStyle = m.styles.InputFocused
	}
	b.WriteString(inputStyle.Width(50).Render(m.configInputs[0].View()))
	b.WriteString("\n\n")
	
	// Folder separator
	b.WriteString(m.styles.InputLabel.Render("Folder Separator:"))
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Character used to create virtual folders"))
	b.WriteString("\n")
	inputStyle = m.styles.Input
	if m.configFocus == 1 {
		inputStyle = m.styles.InputFocused
	}
	b.WriteString(inputStyle.Width(10).Render(m.configInputs[1].View()))
	b.WriteString("\n")
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewConfigTemplates() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("📝 Code Templates"))
	b.WriteString("\n\n")
	
	if len(m.config.Templates) == 0 {
		b.WriteString(m.styles.SubtleText().Render("No templates configured"))
	} else {
		for i, tpl := range m.config.Templates {
			icon := "🔑"
			line := fmt.Sprintf("%s %s", icon, tpl.Title)
			if i == m.templateListCursor {
				line = m.styles.ListSelected.Width(50).Render("▶ " + line)
			} else {
				line = m.styles.ListItem.Width(50).Render("  " + line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	
	// Add new option
	b.WriteString("\n")
	addLine := "➕ Add new template"
	if m.templateListCursor == len(m.config.Templates) {
		addLine = m.styles.ListSelected.Width(50).Render("▶ " + addLine)
	} else {
		addLine = m.styles.ListItem.Width(50).Render("  " + addLine)
	}
	b.WriteString(addLine)
	b.WriteString("\n")
	
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Variables: {{.SecretName}}, {{.FullSecretName}}, {{.ProjectID}}"))
	
	return b.String()
}

func (m Model) viewConfigTemplateEdit() string {
	var b strings.Builder
	
	title := "📝 Edit Template"
	if m.isNewTemplate {
		title = "📝 New Template"
	}
	b.WriteString(m.styles.DialogTitle.Render(title))
	b.WriteString("\n\n")
	
	// Title input
	titleLabel := "Title:"
	if m.templateFocus == 0 {
		titleLabel = "▶ Title:"
	}
	b.WriteString(m.styles.InputLabel.Render(titleLabel))
	b.WriteString("\n")
	inputStyle := m.styles.Input
	if m.templateFocus == 0 {
		inputStyle = m.styles.InputFocused
	}
	b.WriteString(inputStyle.Width(50).Render(m.templateTitleInput.View()))
	b.WriteString("\n\n")
	
	// Code textarea
	codeLabel := "Code Template:"
	if m.templateFocus == 1 {
		codeLabel = "▶ Code Template:"
	}
	b.WriteString(m.styles.InputLabel.Render(codeLabel))
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Variables: {{.SecretName}}, {{.FullSecretName}}, {{.ProjectID}}"))
	b.WriteString("\n")
	b.WriteString(m.templateCodeArea.View())
	b.WriteString("\n")
	
	return b.String()
}

func (m Model) viewConfigRecentProjects() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("🕐 Recent Projects"))
	b.WriteString("\n\n")
	
	if len(m.config.RecentProjects) == 0 {
		b.WriteString(m.styles.SubtleText().Render("No recent projects"))
	} else {
		for i, project := range m.config.RecentProjects {
			icon := "📁"
			if project == m.config.ProjectID {
				icon = "✓"
			}
			line := fmt.Sprintf("%s %s", icon, project)
			if i == m.recentProjectsCursor {
				line = m.styles.ListSelected.Width(50).Render("▶ " + line)
			} else {
				line = m.styles.ListItem.Width(50).Render("  " + line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Press Enter to switch, d to remove"))
	
	return b.String()
}

func (m Model) viewConfigSecurity() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("🔒 Security Settings"))
	b.WriteString("\n\n")
	
	// Section: Clipboard
	b.WriteString(m.styles.InputLabel.Render("📋 Clipboard"))
	b.WriteString("\n")
	
	// Option 0: Auto-clear clipboard
	autoClearIcon := "○"
	autoClearStatus := "Disabled"
	if m.config.Clipboard.AutoClear {
		autoClearIcon = "✓"
		autoClearStatus = "Enabled"
	}
	line0 := fmt.Sprintf("%s Auto-clear: %s", autoClearIcon, autoClearStatus)
	if m.securityCursor == 0 {
		line0 = m.styles.ListSelected.Width(55).Render("▶ " + line0)
	} else {
		line0 = m.styles.ListItem.Width(55).Render("  " + line0)
	}
	b.WriteString(line0)
	b.WriteString("\n")
	
	// Option 1: Clipboard Timeout
	line1 := fmt.Sprintf("⏱  Clear timeout: %d seconds", m.config.Clipboard.TimeoutSeconds)
	if m.securityCursor == 1 {
		line1 = m.styles.ListSelected.Width(55).Render("▶ " + line1)
	} else {
		line1 = m.styles.ListItem.Width(55).Render("  " + line1)
	}
	b.WriteString(line1)
	b.WriteString("\n\n")
	
	// Section: Audit Logging
	b.WriteString(m.styles.InputLabel.Render("📝 Audit Logging"))
	b.WriteString("\n")
	
	// Option 2: Audit enabled
	auditIcon := "○"
	auditStatus := "Disabled"
	if m.config.Audit.Enabled {
		auditIcon = "✓"
		auditStatus = "Enabled"
	}
	line2 := fmt.Sprintf("%s Audit logging: %s", auditIcon, auditStatus)
	if m.securityCursor == 2 {
		line2 = m.styles.ListSelected.Width(55).Render("▶ " + line2)
	} else {
		line2 = m.styles.ListItem.Width(55).Render("  " + line2)
	}
	b.WriteString(line2)
	b.WriteString("\n")
	
	// Option 3: Audit retention
	line3 := fmt.Sprintf("📅 Log retention: %d days", m.config.Audit.MaxAgeDays)
	if m.securityCursor == 3 {
		line3 = m.styles.ListSelected.Width(55).Render("▶ " + line3)
	} else {
		line3 = m.styles.ListItem.Width(55).Render("  " + line3)
	}
	b.WriteString(line3)
	b.WriteString("\n")
	
	// Option 4: View audit logs
	line4 := "👁  View audit logs →"
	if m.securityCursor == 4 {
		line4 = m.styles.ListSelected.Width(55).Render("▶ " + line4)
	} else {
		line4 = m.styles.ListItem.Width(55).Render("  " + line4)
	}
	b.WriteString(line4)
	b.WriteString("\n\n")
	
	// Section: Session Security
	b.WriteString(m.styles.InputLabel.Render("⏰ Session Security"))
	b.WriteString("\n")
	
	// Option 5: Session lock enabled
	lockIcon := "○"
	lockStatus := "Disabled"
	if m.config.Session.LockOnTimeout {
		lockIcon = "✓"
		lockStatus = "Enabled"
	}
	line5 := fmt.Sprintf("%s Lock on timeout: %s", lockIcon, lockStatus)
	if m.securityCursor == 5 {
		line5 = m.styles.ListSelected.Width(55).Render("▶ " + line5)
	} else {
		line5 = m.styles.ListItem.Width(55).Render("  " + line5)
	}
	b.WriteString(line5)
	b.WriteString("\n")
	
	// Option 6: Inactivity timeout
	timeoutText := "Disabled"
	if m.config.Session.InactivityTimeout > 0 {
		timeoutText = fmt.Sprintf("%d minutes", m.config.Session.InactivityTimeout)
	}
	line6 := fmt.Sprintf("⏱  Inactivity timeout: %s", timeoutText)
	if m.securityCursor == 6 {
		line6 = m.styles.ListSelected.Width(55).Render("▶ " + line6)
	} else {
		line6 = m.styles.ListItem.Width(55).Render("  " + line6)
	}
	b.WriteString(line6)
	b.WriteString("\n\n")
	
	// Show audit log path if enabled
	if m.config.Audit.Enabled && m.auditLogger != nil {
		logPath := m.auditLogger.GetFilePath()
		if logPath != "" {
			b.WriteString(m.styles.SubtleText().Render("Log: " + logPath))
			b.WriteString("\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render("Press Enter/Space to toggle, ↑↓ to navigate"))
	
	return m.styles.Dialog.Render(b.String())
}

func (m Model) viewAuditLog() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("📜 Audit Log Viewer"))
	b.WriteString("\n\n")
	
	if len(m.auditLogLines) == 0 {
		b.WriteString(m.styles.SubtleText().Render("No log entries found"))
		b.WriteString("\n")
	} else {
		// Header
		header := fmt.Sprintf("%-19s %s %-12s %-25s %-30s", "TIMESTAMP", "R", "EVENT", "USER", "SECRET")
		b.WriteString(m.styles.InputLabel.Render(header))
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render(strings.Repeat("─", 95)))
		b.WriteString("\n")
		
		// Calculate visible lines
		visibleLines := m.height - 12
		if visibleLines < 5 {
			visibleLines = 5
		}
		
		// Show logs with offset
		endIdx := m.auditLogOffset + visibleLines
		if endIdx > len(m.auditLogLines) {
			endIdx = len(m.auditLogLines)
		}
		
		for i := m.auditLogOffset; i < endIdx; i++ {
			line := m.auditLogLines[i]
			formatted := audit.FormatLogEntry(line)
			b.WriteString(formatted)
			b.WriteString("\n")
		}
		
		// Scroll indicator
		b.WriteString("\n")
		total := len(m.auditLogLines)
		showing := endIdx - m.auditLogOffset
		b.WriteString(m.styles.SubtleText().Render(
			fmt.Sprintf("Showing %d-%d of %d entries (most recent first)", 
				m.auditLogOffset+1, m.auditLogOffset+showing, total),
		))
	}
	
	return b.String()
}

// ASCII art logo for splash screens
const asciiLogo = `
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
`

func (m Model) viewSplash(message string, icon string, subtitle string) string {
	var b strings.Builder
	
	// Center the logo
	logoStyle := lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true)
	
	b.WriteString(logoStyle.Render(asciiLogo))
	b.WriteString("\n")
	
	// Icon and message centered
	if icon != "" {
		iconMsg := fmt.Sprintf("%s  %s", icon, message)
		msgStyle := lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)
		b.WriteString(msgStyle.Render(iconMsg))
		b.WriteString("\n\n")
	}
	
	// Subtitle
	if subtitle != "" {
		subtitleStyle := lipgloss.NewStyle().
			Foreground(ColorTextMuted)
		b.WriteString(subtitleStyle.Render(subtitle))
	}
	
	return b.String()
}

func (m Model) viewLocked() string {
	timeout := m.config.Session.InactivityTimeout
	subtitle := fmt.Sprintf("Session locked after %d minutes of inactivity\n\n⚠️  Sensitive data cleared from memory\n\nPress ENTER or SPACE to unlock", timeout)
	return m.viewSplash("SESSION LOCKED", "🔒", subtitle)
}

func (m Model) viewProjectSwitch() string {
	var b strings.Builder
	
	b.WriteString(m.styles.DialogTitle.Render("🔀 Switch Project"))
	b.WriteString("\n\n")
	
	// Current project indicator
	b.WriteString(m.styles.SubtleText().Render("Current: "))
	b.WriteString(m.styles.StatusInfo.Render(m.config.ProjectID))
	b.WriteString("\n\n")
	
	// Search/input field
	b.WriteString(m.styles.InputLabel.Render("Search or enter new project ID:"))
	b.WriteString("\n")
	b.WriteString(m.styles.InputFocused.Width(50).Render(m.projectSwitchInput.View()))
	b.WriteString("\n\n")
	
	filterText := m.projectSwitchInput.Value()
	filteredProjects := m.getFilteredProjects(filterText)
	
	// Check if typed value is a new project
	isNewProject := filterText != "" && !m.projectExists(filterText)
	
	// Show option to use typed value if there's input
	cursorIdx := 0
	if filterText != "" {
		var line string
		if isNewProject {
			line = fmt.Sprintf("➕ Add new project: \"%s\"", filterText)
		} else {
			line = fmt.Sprintf("➡️  Switch to: \"%s\"", filterText)
		}
		if m.projectSwitchCursor == 0 {
			line = m.styles.ListSelected.Width(55).Render(line)
		} else {
			line = m.styles.ListItem.Width(55).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		cursorIdx = 1
	}
	
	// Saved projects list
	if len(filteredProjects) > 0 {
		b.WriteString("\n")
		label := fmt.Sprintf("Saved projects (%d):", len(m.config.RecentProjects))
		b.WriteString(m.styles.SubtleText().Render(label))
		b.WriteString("\n")
		
		// Show max 15 projects to avoid overflow
		maxShow := 15
		if len(filteredProjects) < maxShow {
			maxShow = len(filteredProjects)
		}
		
		for i := 0; i < maxShow; i++ {
			project := filteredProjects[i]
			icon := "  📁"
			if project == m.config.ProjectID {
				icon = "  ✓ "
			}
			line := fmt.Sprintf("%s %s", icon, project)
			
			listIdx := cursorIdx + i
			if m.projectSwitchCursor == listIdx {
				line = m.styles.ListSelected.Width(55).Render("▶" + line[1:])
			} else {
				line = m.styles.ListItem.Width(55).Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		
		if len(filteredProjects) > maxShow {
			b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("  ... and %d more (type to filter)", len(filteredProjects)-maxShow)))
			b.WriteString("\n")
		}
	} else if filterText != "" && len(m.config.RecentProjects) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render("No matching projects in history"))
		b.WriteString("\n")
	} else if len(m.config.RecentProjects) == 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.SubtleText().Render("No saved projects yet"))
		b.WriteString("\n")
	}
	
	return m.styles.Dialog.Render(b.String())
}

// projectExists checks if a project ID exists in saved projects
func (m Model) projectExists(projectID string) bool {
	for _, p := range m.config.RecentProjects {
		if strings.EqualFold(p, projectID) {
			return true
		}
	}
	return false
}

func (m Model) viewFilter() string {
	var b strings.Builder
	
	b.WriteString(m.styles.InputLabel.Render("🔍 Filter:"))
	b.WriteString("\n")
	b.WriteString(m.styles.InputFocused.Width(50).Render(m.filterInput.View()))
	b.WriteString("\n\n")
	
	// Show preview of filtered results
	count := len(m.displayItems)
	b.WriteString(m.styles.SubtleText().Render(fmt.Sprintf("%d items matching", count)))
	
	return b.String()
}

func (m Model) viewReveal() string {
	var b strings.Builder
	
	// Header with secret info
	b.WriteString(m.styles.DialogTitle.Render("🔓 Secret Value"))
	b.WriteString("\n\n")
	
	// Secret metadata
	if m.selectedSecret != nil {
		b.WriteString(m.styles.DetailLabel.Render("Secret: "))
		b.WriteString(m.styles.DetailValue.Render(m.selectedSecret.Name))
		b.WriteString("\n")
	}
	b.WriteString(m.styles.DetailLabel.Render("Version: "))
	b.WriteString(m.styles.DetailVersion.Render(m.revealVersion))
	b.WriteString("\n")
	
	// Convert to string only for display (minimize lifetime)
	displayValue := m.getRevealedValueString()
	
	// Size info
	lines := strings.Split(displayValue, "\n")
	lineCount := len(lines)
	byteCount := len(m.revealedValue)
	sizeInfo := fmt.Sprintf("%d bytes", byteCount)
	if lineCount > 1 {
		sizeInfo = fmt.Sprintf("%d lines, %d bytes", lineCount, byteCount)
	}
	b.WriteString(m.styles.DetailLabel.Render("Size: "))
	b.WriteString(m.styles.SubtleText().Render(sizeInfo))
	b.WriteString("\n\n")
	
	// Separator
	separator := strings.Repeat("─", 60)
	b.WriteString(m.styles.SubtleText().Render(separator))
	b.WriteString("\n\n")
	
	// Content with line numbers for multiline
	maxHeight := m.height - 16 // Leave room for header/footer
	if maxHeight < 5 {
		maxHeight = 5
	}
	
	if lineCount == 1 {
		// Single line - show as is with nice formatting
		b.WriteString(m.styles.CodeBlock.Width(m.width - 10).Render(displayValue))
	} else {
		// Multi-line - show with line numbers
		var contentBuilder strings.Builder
		displayLines := lines
		truncated := false
		
		if lineCount > maxHeight {
			displayLines = lines[:maxHeight]
			truncated = true
		}
		
		// Calculate line number width
		lineNumWidth := len(fmt.Sprintf("%d", lineCount))
		
		for i, line := range displayLines {
			lineNum := fmt.Sprintf("%*d", lineNumWidth, i+1)
			contentBuilder.WriteString(
				m.styles.SubtleText().Render(lineNum + " │ "),
			)
			// Highlight lines that look like env vars
			if strings.Contains(line, "=") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					contentBuilder.WriteString(m.styles.ListFolder.Render(parts[0]))
					contentBuilder.WriteString(m.styles.SubtleText().Render("="))
					contentBuilder.WriteString(m.styles.DetailSecret.Render(parts[1]))
				} else {
					contentBuilder.WriteString(line)
				}
			} else if strings.HasPrefix(strings.TrimSpace(line), "#") {
				// Comments in subtle color
				contentBuilder.WriteString(m.styles.SubtleText().Render(line))
			} else {
				contentBuilder.WriteString(line)
			}
			contentBuilder.WriteString("\n")
		}
		
		if truncated {
			contentBuilder.WriteString(m.styles.StatusWarning.Render(
				fmt.Sprintf("\n... %d more lines (content truncated)", lineCount-maxHeight),
			))
		}
		
		b.WriteString(contentBuilder.String())
	}
	
	b.WriteString("\n")
	b.WriteString(m.styles.SubtleText().Render(separator))
	b.WriteString("\n\n")
	
	// Footer hints
	b.WriteString(m.styles.SubtleText().Render("Press "))
	b.WriteString(m.styles.FooterKey.Render("Esc"))
	b.WriteString(m.styles.SubtleText().Render(" to hide"))
	
	return b.String()
}

func (m Model) renderBreadcrumb() string {
	parts := []string{"🏠"}
	parts = append(parts, m.currentPath...)
	
	var rendered []string
	for i, p := range parts {
		if i == len(parts)-1 {
			rendered = append(rendered, m.styles.BreadcrumbItem.Bold(true).Render(p))
		} else {
			rendered = append(rendered, m.styles.BreadcrumbItem.Render(p))
		}
	}
	
	return m.styles.Breadcrumb.Render(
		strings.Join(rendered, m.styles.BreadcrumbSep.Render(" › ")),
	)
}

// SubtleText returns a subtle text style
func (s *Styles) SubtleText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorTextMuted)
}

