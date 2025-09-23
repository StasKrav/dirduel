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

	leftDir    string
	rightDir   string
	leftFiles  []os.DirEntry
	rightFiles []os.DirEntry

	cursorLeft  int
	cursorRight int
	offsetLeft  int
	offsetRight int
	activePane  string

	// терминал
	termInput   string
	termCursor  int
	termOutput  []string
	termHistory []string
	historyPos  int
	termOffset  int // для скролла PageUp/PageDown
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
		historyPos:  -1,
		termOffset:  0,
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

		case "tab":
			if m.activePane == "left" {
				m.activePane = "right"
			} else if m.activePane == "right" {
				m.activePane = "terminal"
			} else {
				m.activePane = "left"
			}

		// ---------- left ----------
		case "left":
			if m.activePane == "terminal" {
				if m.termCursor > 0 {
					m.termCursor--
				}
			} else if m.activePane == "left" {
				m.leftDir = parentDir(m.leftDir)
				m.leftFiles, _ = os.ReadDir(m.leftDir)
				m.cursorLeft, m.offsetLeft = 0, 0
			} else if m.activePane == "right" {
				m.rightDir = parentDir(m.rightDir)
				m.rightFiles, _ = os.ReadDir(m.rightDir)
				m.cursorRight, m.offsetRight = 0, 0
			}

		// ---------- right ----------
		case "right":
			if m.activePane == "terminal" {
				if m.termCursor < len(m.termInput) {
					m.termCursor++
				}
			} else if m.activePane == "left" {
				m = enterItem(m, true)
			} else if m.activePane == "right" {
				m = enterItem(m, false)
			}

		// ---------- up ----------
		case "up":
			if m.activePane == "left" && m.cursorLeft > 0 {
				m.cursorLeft--
				if m.cursorLeft < m.offsetLeft {
					m.offsetLeft--
				}
			} else if m.activePane == "right" && m.cursorRight > 0 {
				m.cursorRight--
				if m.cursorRight < m.offsetRight {
					m.offsetRight--
				}
			} else if m.activePane == "terminal" {
				if m.historyPos < len(m.termHistory)-1 {
					m.historyPos++
					m.termInput = m.termHistory[len(m.termHistory)-1-m.historyPos]
					m.termCursor = len(m.termInput)
				}
			}

		// ---------- down ----------
		case "down":
			if m.activePane == "left" && m.cursorLeft < len(m.leftFiles)-1 {
				m.cursorLeft++
				if m.cursorLeft >= m.offsetLeft+(m.height/2+4-2) {
					m.offsetLeft++
				}
			} else if m.activePane == "right" && m.cursorRight < len(m.rightFiles)-1 {
				m.cursorRight++
				if m.cursorRight >= m.offsetRight+(m.height/2+4-2) {
					m.offsetRight++
				}
			} else if m.activePane == "terminal" {
				if m.historyPos > 0 {
					m.historyPos--
					m.termInput = m.termHistory[len(m.termHistory)-1-m.historyPos]
					m.termCursor = len(m.termInput)
				} else {
					m.historyPos = -1
					m.termInput = ""
					m.termCursor = 0
				}
			}

		// ---------- скролл терминала ----------
		case "pgup":
			if m.termOffset < len(m.termOutput)-1 {
				m.termOffset++
			}
		case "pgdown":
			if m.termOffset > 0 {
				m.termOffset--
			}

		// ---------- работа с терминалом ----------
		case "enter":
			if m.activePane == "terminal" {
				cmd := strings.TrimSpace(m.termInput)
				if cmd != "" {
					m.termOutput = append(m.termOutput, "> "+cmd)
					m.termHistory = append(m.termHistory, cmd)
				}
				m.termInput = ""
				m.termCursor = 0
				m.historyPos = -1
				m.termOffset = 0
			}
		case "backspace":
			if m.activePane == "terminal" && m.termCursor > 0 {
				m.termInput = m.termInput[:m.termCursor-1] + m.termInput[m.termCursor:]
				m.termCursor--
			}

		default:
			if m.activePane == "terminal" {
				if msg.Type == tea.KeyRunes {
					r := msg.Runes[0]
					m.termInput = m.termInput[:m.termCursor] + string(r) + m.termInput[m.termCursor:]
					m.termCursor++
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Загрузка..."
	}

	panelW := m.width/2 - 2
	panelH := m.height/2 + 2
	termH := m.height - panelH - 2

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, m.activePane == "left", panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, m.activePane == "right", panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	term := renderTerminal(m, m.width, termH)
	return ui + "\n" + term
}

func renderPanel(path string, files []os.DirEntry, cursor, offset int, active bool, w, h int) string {
	border := "-"
	color := gray
	if active {
		border = "="
		color = green
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset + "\n")

	visibleFiles := files
	if len(files) > h-2 {
		end := offset + (h - 2)
		if end > len(files) {
			end = len(files)
		}
		visibleFiles = files[offset:end]
	}

	for i := 0; i < h-2; i++ {
		var line string
		if i < len(visibleFiles) {
			idx := offset + i
			name := visibleFiles[i].Name()
			if visibleFiles[i].IsDir() {
				name += "/"
			}
			line = fmt.Sprintf(" %s", name)
			if idx == cursor {
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

func renderTerminal(m model, w, h int) string {
	border := "-"
	color := gray
	if m.activePane == "terminal" {
		border = "="
		color = green
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset + "\n")

	// вывод с учётом прокрутки
	visible := h - 2
	start := len(m.termOutput) - visible - m.termOffset
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.termOutput) {
		end = len(m.termOutput)
	}
	visibleLines := m.termOutput[start:end]

	for i := 0; i < visible; i++ {
		var line string
		if i < len(visibleLines) {
			line = visibleLines[i]
		}
		if len(stripANSI(line)) > w-2 {
			line = line[:w-2]
		}
		b.WriteString(color + "|" + reset + line +
			strings.Repeat(" ", w-2-len(stripANSI(line))) +
			color + "|" + reset + "\n")
	}

	// строка ввода
	input := m.termInput
	cursor := m.termCursor
	inputWithCursor := input[:cursor] + yellow + "▌" + reset + input[cursor:]
	if len(stripANSI(inputWithCursor)) > w-2 {
		inputWithCursor = inputWithCursor[len(inputWithCursor)-(w-2):]
	}
	b.WriteString(color + "|" + reset + "> "+inputWithCursor+
		strings.Repeat(" ", w-2-len(stripANSI("> "+inputWithCursor)))+
		color + "|" + reset + "\n")

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
			m.cursorLeft, m.offsetLeft = 0, 0
		} else {
			m.rightDir = path
			m.rightFiles = newFiles
			m.cursorRight, m.offsetRight = 0, 0
		}
	} else {
		m.termOutput = append(m.termOutput, fmt.Sprintf("Запуск файла: %s", path))
	}

	return m
}

func parentDir(path string) string {
	return filepath.Dir(path)
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
