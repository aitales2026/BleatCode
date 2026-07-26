package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/penta/BleatCode/internal/agent"
)

const renderInterval = 150 * time.Millisecond

// Messages
type chunkMsg struct{ content string }
type renderTickMsg struct{}
type doneMsg struct{}

// focusTarget tracks which component has focus.
type focusTarget int

const (
	focusInput focusTarget = iota
	focusViewport
)

type Model struct {
	// Output area
	viewport    viewport.Model
	glam        *glamour.TermRenderer
	rawOutput   string
	rendered    string
	styledLogo  string
	dirty       bool
	ready       bool
	width       int
	height      int
	tickStarted bool

	// Input area
	input textinput.Model

	// Status bar
	modelName string
	workDir   string

	// Agent integration
	loop *agent.Loop
	ch   chan string // current turn's channel
	busy bool

	// Token usage
	lastUsage agent.Usage

	// Focus
	focus focusTarget
}

func listenCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		content, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return chunkMsg{content: content}
	}
}

func renderTick() tea.Cmd {
	return tea.Tick(renderInterval, func(t time.Time) tea.Msg {
		return renderTickMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	return m.input.Focus()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		bottomHeight := 3 // separator + input + status bar
		glamourRenderWidth := msg.Width - 3
		glam, err := glamour.NewTermRenderer(
			glamour.WithStyles(customGlamourStyle()),
			glamour.WithWordWrap(glamourRenderWidth),
			glamour.WithPreservedNewLines(),
		)
		if err == nil {
			m.glam = glam
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-bottomHeight)
			m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
			m.ready = true
			// Render initial content (logo) immediately
			if m.dirty && m.glam != nil {
				out, err := m.glam.Render(m.rawOutput)
				if err != nil {
					out = m.rawOutput
				}
				m.rendered = m.styledLogo + strings.TrimRightFunc(out, func(r rune) bool { return r == '\n' || r == '\r' })
				m.viewport.SetContent(m.rendered)
				m.viewport.GotoBottom()
				m.dirty = false
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - bottomHeight
			m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
		}
		return m, nil

	case chunkMsg:
		m.rawOutput += msg.content
		m.dirty = true
		if !m.tickStarted {
			m.tickStarted = true
			return m, tea.Batch(listenCmd(m.ch), renderTick())
		}
		return m, listenCmd(m.ch)

	case renderTickMsg:
		if m.dirty && m.ready && m.glam != nil {
			wasAtBottom := m.viewport.ScrollPercent() >= 0.99
			out, err := m.glam.Render(m.rawOutput)
			if err != nil {
				out = m.rawOutput
			}
			m.rendered = m.styledLogo + strings.TrimRightFunc(out, func(r rune) bool { return r == '\n' || r == '\r' })
			m.viewport.SetContent(m.rendered)
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
			m.dirty = false
		}
		return m, renderTick()

	case doneMsg:
		m.busy = false
		m.tickStarted = false
		m.ch = nil
		m.lastUsage = m.loop.GetUsage()
		// Final render
		if m.ready && m.glam != nil {
			out, err := m.glam.Render(m.rawOutput)
			if err != nil {
				out = m.rawOutput
			}
			m.rendered = m.styledLogo + strings.TrimRightFunc(out, func(r rune) bool { return r == '\n' || r == '\r' })
			m.viewport.SetContent(m.rendered)
			m.viewport.GotoBottom()
		}
		// Append separator for next turn
		m.rawOutput += "\n\n---\n\n"
		m.dirty = true
		// Re-focus input
		m.input.Focus()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			if m.busy {
				return m, nil
			}
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				m.input.SetValue("")
				return m, nil
			}
			if strings.ToLower(query) == "q" || strings.ToLower(query) == "exit" {
				return m, tea.Quit
			}
			// Start agent turn
			m.input.SetValue("")
			m.busy = true
			m.ch = make(chan string, 64)
			// Append user query to output
			m.rawOutput += fmt.Sprintf("**You:** %s\n\n", query)
			m.dirty = true
			go m.loop.Run(context.Background(), query, m.ch)
			return m, listenCmd(m.ch)

		// Scroll controls (always available, don't conflict with textinput)
		case "alt+up":
			m.viewport.LineUp(1)
			return m, nil
		case "alt+down":
			m.viewport.LineDown(1)
			return m, nil
		case "ctrl+u":
			m.viewport.HalfViewUp()
			return m, nil
		case "ctrl+d":
			m.viewport.HalfViewDown()
			return m, nil
		case "pgup":
			m.viewport.ViewUp()
			return m, nil
		case "pgdown":
			m.viewport.ViewDown()
			return m, nil
		}
	}

	// Always pass to viewport for mouse scroll handling
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)

	// Pass to textinput when focused, but skip mouse events
	if m.focus == focusInput {
		if _, isMouse := msg.(tea.MouseMsg); !isMouse {
			var tiCmd tea.Cmd
			m.input, tiCmd = m.input.Update(msg)
			return m, tea.Batch(vpCmd, tiCmd)
		}
	}
	return m, vpCmd
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	viewportView := m.viewport.View()
	separatorView := m.renderSeparator()
	inputView := m.renderInput()
	statusView := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		viewportView,
		separatorView,
		inputView,
		statusView,
	)
}

func (m Model) renderSeparator() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#00bcd4"))
	return style.Render(strings.Repeat("─", m.width))
}

func (m Model) renderInput() string {
	if m.busy {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return style.Render("  " + m.input.Prompt + "... (waiting for response)")
	}
	return m.input.View()
}

func (m Model) renderStatusBar() string {
	left := fmt.Sprintf(" %s ", m.modelName)
	center := fmt.Sprintf(" Usage: Prompt:%d / Completion:%d / Total:%d ", m.lastUsage.PromptTokens, m.lastUsage.CompletionTokens, m.lastUsage.TotalTokens)
	right := fmt.Sprintf(" %s ", m.workDir)

	leftStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d0d0d0")).
		Background(lipgloss.Color("#3b3b3b")).
		Bold(true)

	centerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50fa7b")).
		Background(lipgloss.Color("#3b3b3b"))

	rightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a0a0a0")).
		Background(lipgloss.Color("#3b3b3b"))

	leftRendered := leftStyle.Render(left)
	centerRendered := centerStyle.Render(center)
	rightRendered := rightStyle.Render(right)

	gap := m.width - lipgloss.Width(leftRendered) - lipgloss.Width(centerRendered) - lipgloss.Width(rightRendered)
	if gap < 0 {
		gap = 0
	}

	midStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#3b3b3b"))

	return leftRendered + midStyle.Render(strings.Repeat(" ", gap/2)) + centerRendered + midStyle.Render(strings.Repeat(" ", gap-gap/2)) + rightRendered
}

func customGlamourStyle() ansi.StyleConfig {
	style := styles.DraculaStyleConfig

	// H1: orange bold underline
	style.H1 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:    " ",
			Suffix:    " ",
			Color:     strPtr("#ffb86c"),
			Bold:      boolPtr(true),
			Underline: boolPtr(true),
		},
	}
	// H2: purple bold
	style.H2 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Color:  strPtr("#bd93f9"),
			Bold:   boolPtr(true),
		},
	}
	// H3: cyan bold
	style.H3 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Color:  strPtr("#8be9fd"),
			Bold:   boolPtr(true),
		},
	}
	// H4: green bold
	style.H4 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Color:  strPtr("#50fa7b"),
			Bold:   boolPtr(true),
		},
	}
	// H5: pink bold
	style.H5 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Color:  strPtr("#ff79c6"),
			Bold:   boolPtr(true),
		},
	}
	// H6: gray
	style.H6 = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Color:  strPtr("#6272A4"),
			Bold:   boolPtr(false),
		},
	}
	// Horizontal rule: Dracula cyan
	style.HorizontalRule = ansi.StylePrimitive{
		Color:  strPtr("#8be9fd"),
		Format: "\n────────────────────────────────────────\n",
	}
	// Chroma Error: match code block background to avoid red highlight on box-drawing chars
	style.CodeBlock.Chroma.Error.BackgroundColor = strPtr("#282a36")
	return style
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
