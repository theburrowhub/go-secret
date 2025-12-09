package ui

import "github.com/charmbracelet/lipgloss"

// Color palette - Darcula theme (JetBrains inspired)
var (
	// Base colors
	ColorBackground  = lipgloss.Color("#2B2B2B")
	ColorSurface     = lipgloss.Color("#3C3F41")
	ColorSurfaceAlt  = lipgloss.Color("#313335")
	ColorBorder      = lipgloss.Color("#515151")
	
	// Text colors
	ColorText        = lipgloss.Color("#A9B7C6")
	ColorTextMuted   = lipgloss.Color("#808080")
	ColorTextBright  = lipgloss.Color("#FFFFFF")
	
	// Accent colors
	ColorOrange      = lipgloss.Color("#CC7832")  // Keywords
	ColorGreen       = lipgloss.Color("#6A8759")  // Strings
	ColorBlue        = lipgloss.Color("#6897BB")  // Numbers
	ColorYellow      = lipgloss.Color("#FFC66D")  // Functions
	ColorPurple      = lipgloss.Color("#9876AA")  // Constants
	ColorCyan        = lipgloss.Color("#299999")  // Links
	
	// Status colors
	ColorRed         = lipgloss.Color("#FF6B68")
	ColorGreenBright = lipgloss.Color("#499C54")
	ColorYellowWarn  = lipgloss.Color("#BBB529")
	
	// Selection
	ColorSelection   = lipgloss.Color("#214283")
)

// Styles contains all the application styles
type Styles struct {
	// Base styles
	App          lipgloss.Style
	Header       lipgloss.Style
	Footer       lipgloss.Style
	FooterKey    lipgloss.Style
	FooterDesc   lipgloss.Style
	Content      lipgloss.Style
	
	// List styles
	ListTitle    lipgloss.Style
	ListItem     lipgloss.Style
	ListSelected lipgloss.Style
	ListFolder   lipgloss.Style
	ListSecret   lipgloss.Style
	
	// Detail styles
	DetailTitle   lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style
	DetailVersion lipgloss.Style
	DetailSecret  lipgloss.Style
	
	// Input styles
	Input        lipgloss.Style
	InputLabel   lipgloss.Style
	InputFocused lipgloss.Style
	
	// Status styles
	StatusInfo    lipgloss.Style
	StatusError   lipgloss.Style
	StatusSuccess lipgloss.Style
	StatusWarning lipgloss.Style
	
	// Dialog styles
	Dialog       lipgloss.Style
	DialogTitle  lipgloss.Style
	DialogButton lipgloss.Style
	
	// Breadcrumb
	Breadcrumb     lipgloss.Style
	BreadcrumbSep  lipgloss.Style
	BreadcrumbItem lipgloss.Style
	
	// Code block
	CodeBlock lipgloss.Style
}

// NewStyles creates the default styles
func NewStyles() *Styles {
	return &Styles{
		App: lipgloss.NewStyle().
			Background(ColorBackground),
		
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorYellow).
			Background(ColorSurface).
			Padding(0, 2).
			MarginBottom(1),
		
		Footer: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Background(ColorSurface).
			Padding(0, 1),
		
		FooterKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBackground).
			Background(ColorOrange).
			Padding(0, 1).
			MarginRight(0),
		
		FooterDesc: lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorSurfaceAlt).
			Padding(0, 1).
			MarginRight(2),
		
		Content: lipgloss.NewStyle().
			Padding(0, 2),
		
		ListTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorOrange).
			MarginBottom(1),
		
		ListItem: lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2),
		
		ListSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTextBright).
			Background(ColorSelection).
			PaddingLeft(2),
		
		ListFolder: lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true),
		
		ListSecret: lipgloss.NewStyle().
			Foreground(ColorText),
		
		DetailTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorYellow).
			MarginBottom(1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorBorder),
		
		DetailLabel: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Width(15),
		
		DetailValue: lipgloss.NewStyle().
			Foreground(ColorText),
		
		DetailVersion: lipgloss.NewStyle().
			Foreground(ColorBlue),
		
		DetailSecret: lipgloss.NewStyle().
			Foreground(ColorGreen),
		
		Input: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1),
		
		InputLabel: lipgloss.NewStyle().
			Foreground(ColorOrange).
			Bold(true),
		
		InputFocused: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorYellow).
			Padding(0, 1),
		
		StatusInfo: lipgloss.NewStyle().
			Foreground(ColorBlue),
		
		StatusError: lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true),
		
		StatusSuccess: lipgloss.NewStyle().
			Foreground(ColorGreenBright),
		
		StatusWarning: lipgloss.NewStyle().
			Foreground(ColorYellowWarn),
		
		Dialog: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2),
		
		DialogTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorYellow).
			MarginBottom(1),
		
		DialogButton: lipgloss.NewStyle().
			Foreground(ColorBackground).
			Background(ColorOrange).
			Padding(0, 2).
			MarginRight(1),
		
		Breadcrumb: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			MarginBottom(1),
		
		BreadcrumbSep: lipgloss.NewStyle().
			Foreground(ColorBorder),
		
		BreadcrumbItem: lipgloss.NewStyle().
			Foreground(ColorYellow),
		
		CodeBlock: lipgloss.NewStyle().
			Foreground(ColorGreen).
			Background(ColorSurfaceAlt).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder),
	}
}
