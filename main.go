package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	reset  = "\033[0m"
	green  = "\033[32m"
	gray   = "\033[90m"
	yellow = "\033[33m"
)

type model struct {
	width, height int

	leftDir   string
	rightDir  string
	leftFiles []os.DirEntry
	rightFiles []os.DirEntry

	cursorLeft  int
	cursorRight int
	activePane  string

	showTerminal bool
	termInput    string
	termOutput   []string
}

func initialModel() model {
	wd, _ := os.Getwd()
	files, _ := os.ReadDir(wd)
	return model{
		activePane:  "left",
		leftDir:     wd,
		rightDir:    wd,
		leftFiles:   files,
		rightFiles:  files,
		termOutput:  []string{"Терминал готов"},
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+q":
			return m, tea.Quit

		case "left":
			if m.activePane == "left" {
				m.leftDir = parentDir(m.leftDir)
				m.leftFiles, _ = os.ReadDir(m.leftDir)
				m.cursorLeft = 0
			} else {
				m.rightDir = parentDir(m.rightDir)
				m.rightFiles, _ = os.ReadDir(m.rightDir)
				m.cursorRight = 0
			}

		case "right":
			if m.activePane == "left" {
				m = enterItem(m, true)
			} else {
				m = enterItem(m, false)
			}

		case "up":
			if m.activePane == "left" && m.cursorLeft > 0 {
				m.cursorLeft--
			} else if m.activePane == "right" && m.cursorRight > 0 {
				m.cursorRight--
			}

		case "down":
			if m.activePane == "left" && m.cursorLeft < len(m.leftFiles)-1 {
				m.cursorLeft++
			} else if m.activePane == "right" && m.cursorRight < len(m.rightFiles)-1 {
				m.cursorRight++
			}

		case "alt+left":
			m.activePane = "left"
		case "alt+right":
			m.activePane = "right"
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Загрузка..."
	}

	panelW := m.width/2 - 2
	panelH := m.height/2 + 4

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.activePane == "left", panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.activePane == "right", panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	return ui
}

func renderPanel(path string, files []os.DirEntry, cursor int, active bool, w, h int) string {
	border := "-"
	color := gray
	if active {
		border = "="
		color = green
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset + "\n")
	for i := 0; i < h-2; i++ {
		var line string
		if i < len(files) {
			name := files[i].Name()
			if files[i].IsDir() {
				name += "/"
			}
			line = fmt.Sprintf(" %s", name)
			if i == cursor {
				line = yellow + ">" + line + reset
			}
		}
		if len(stripANSI(line)) > w-2 {
			line = line[:w-2]
		}
		b.WriteString(color + "|" + reset + line +
			strings.Repeat(" ", w-2-len(stripANSI(line))) +
			color + "|" + reset + "\n")
	}
	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

func enterItem(m model, left bool) model {
	var dir string
	var files []os.DirEntry
	var cursor int

	if left {
		dir = m.leftDir
		files = m.leftFiles
		cursor = m.cursorLeft
	} else {
		dir = m.rightDir
		files = m.rightFiles
		cursor = m.cursorRight
	}

	if cursor >= len(files) {
		return m
	}

	item := files[cursor]
	path := filepath.Join(dir, item.Name())
	if item.IsDir() {
		newFiles, _ := os.ReadDir(path)
		if left {
			m.leftDir = path
			m.leftFiles = newFiles
			m.cursorLeft = 0
		} else {
			m.rightDir = path
			m.rightFiles = newFiles
			m.cursorRight = 0
		}
	} else {
		m.termOutput = append(m.termOutput, fmt.Sprintf("Запуск файла: %s", path))
	}

	return m
}

func parentDir(path string) string {
	parent := filepath.Dir(path)
	return parent
}

func stripANSI(s string) string {
	s = strings.ReplaceAll(s, reset, "")
	s = strings.ReplaceAll(s, green, "")
	s = strings.ReplaceAll(s, gray, "")
	s = strings.ReplaceAll(s, yellow, "")
	return s
}

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Ошибка запуска:", err)
		os.Exit(1)
	}
}
