package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/penta/BleatCode/internal/agent"
)

const logoASCII = `
 ██████╗ ██╗     ███████╗ █████╗ ████████╗ ██████╗  ██████╗  ██████╗  ███████╗
 ██╔══██╗██║     ██╔════╝██╔══██╗╚══██╔══╝██╔═══██╗██╔════╝ ██╔═══██╗ ██╔════╝
 ██████╔╝██║     █████╗  ███████║   ██║   ██║   ██║██║      ██║   ██║ █████╗
 ██╔══██╗██║     ██╔══╝  ██╔══██║   ██║   ██║   ██║██║      ██║   ██║ ██╔══╝
 ██████╔╝███████╗███████╗██║  ██║   ██║   ╚██████╔╝╚██████╗ ╚██████╔╝ ███████╗
 ╚═════╝ ╚══════╝╚══════╝╚═╝  ╚═╝   ╚═╝    ╚═════╝  ╚═════╝  ╚═════╝  ╚══════╝
`

// Run starts the persistent TUI with integrated input, output, and status bar.
func Run(loop *agent.Loop, modelName, workDir string) error {
	input := textinput.New()
	orange := lipgloss.NewStyle().Foreground(lipgloss.Color("#e67e22")).Bold(true)
	input.Prompt = orange.Render("[BleatCode]") + " > "
	input.Placeholder = "Type a message..."
	input.CharLimit = 4096
	input.Focus()

	darkGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#228B22"))
	styledLogo := "\n" + darkGreen.Render(strings.TrimRight(logoASCII, "\n")) + "\n\n"
	welcome := "  Welcome! Type a message to get started. Type 'q' to quit.\n"

	m := Model{
		input:      input,
		modelName:  modelName,
		workDir:    workDir,
		loop:       loop,
		focus:      focusInput,
		styledLogo: styledLogo,
		rawOutput:  welcome,
		dirty:      true,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}
