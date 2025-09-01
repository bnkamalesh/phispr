package tui

// A simple program demonstrating the text area component from the Bubbles
// component library.

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/naughtygopher/errors"
)

const gap = "\n\n"

type (
	errMsg error
)

type model struct {
	viewport    viewport.Model
	messages    []*messages.Message
	textarea    textarea.Model
	senderStyle lipgloss.Style
	memberStyle lipgloss.Style
	currentRoom string
	tui         *TUI
	err         error
}

func (m *model) Messages() string {
	sort.Slice(m.messages, func(i, j int) bool {
		return m.messages[i].ServerReceivedAt.Before(m.messages[j].ServerReceivedAt)
	})

	groupedDates := map[string]struct{}{}
	buff := bytes.NewBuffer(make([]byte, 0, len(m.messages)))
	for _, msg := range m.messages {
		day := msg.ServerReceivedAt.Format(time.DateOnly)
		_, yes := groupedDates[day]
		header := msg.ServerReceivedAt.Format(time.TimeOnly)
		if !yes {
			groupedDates[day] = struct{}{}
			header = msg.ServerReceivedAt.Format(time.DateTime)
		}
		author := msg.Author.Name
		if m.tui.currentMember != nil && author == m.tui.currentMember.User.Name {
			author = m.senderStyle.Render(author)
		}
		fmt.Fprintf(buff, "[%s] %s: %s\n", header, author, msg.Content)
	}
	return buff.String()
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Prompt = "> "
	ta.CharLimit = 512
	ta.SetWidth(25)
	ta.SetHeight(2)

	ta.ShowLineNumbers = false
	ta.Focus()

	vp := viewport.New(30, 5)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		messages:    []*messages.Message{},
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		memberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		textarea:    ta,
		viewport:    vp,
		err:         nil,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.viewport.SetContent(
				lipgloss.NewStyle().Width(m.viewport.Width).Render(m.Messages()),
			)
		}
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.textarea.Value())
			switch val {
			case "/exit":
				return m, tea.Quit

			case "/clear":
				m.messages = make([]*messages.Message, 0, 50)
				m.viewport.SetContent(
					fmt.Sprintf("===== Phispr [%s] ====", m.currentRoom),
				)

			default:
				err := m.tui.SendMessage(val)
				if err != nil {
					m.err = err
					return m, nil
				}
			}

			m.textarea.Reset()
			m.textarea.Focus()
		}

	case *messages.Message:
		m.messages = append(m.messages, msg)
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(m.Messages()))
		m.viewport.GotoBottom()

	case string:
		m.viewport.SetContent(
			lipgloss.NewStyle().Width(m.viewport.Width).Render(m.Messages() + "\n" + msg),
		)
		m.viewport.GotoBottom()

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		str, _ := errors.Message(msg)
		list := make([]string, 0, 5)
		lookup := map[string]struct{}{}
		for _, line := range strings.Split(str, "\n") {
			if line == "" {
				continue
			}
			if _, ok := lookup[line]; !ok {
				lookup[line] = struct{}{}
				list = append(list, line)
			}
		}
		m.viewport.SetContent(strings.Join(list, "\n"))
		m.textarea.Reset()
		return m, tea.Quit
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}
