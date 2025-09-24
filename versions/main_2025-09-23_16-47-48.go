package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
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

	cursorLeft   int
	cursorRight  int
	offsetLeft   int
	offsetRight  int
	activePane   string
	focus        string // "left", "right", "terminal"

	termInput     string
	termOutput    []string
	history       []string
	historyIndex  int
	cursorPos     int
}

func initialModel() model {
	wd, _ := os.Getwd()
	files, _ := os.ReadDir(wd)
	return model{
		activePane:   "left",
		focus:        "left",
		leftDir:      wd,
		rightDir:     wd,
		leftFiles:    files,
		rightFiles:   files,
		termOutput:   []string{"$ "},
		history:      []string{},
		historyIndex: -1,
		cursorPos:    0,
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

		// переключение фокуса
		case "tab":
			if m.focus == "terminal" {
				m.focus = m.activePane
			} else {
				m.focus = "terminal"
				m.cursorPos = len(m.termInput)
			}

		// переключение активной панели
		case "alt+left":
			if m.focus != "terminal" {
				m.activePane = "left"
				m.focus = "left"
			}
		case "alt+right":
			if m.focus != "terminal" {
				m.activePane = "right"
				m.focus = "right"
			}

		// перемещение курсора в панелях
		case "up":
			if m.focus == "left" && m.cursorLeft > 0 {
				m.cursorLeft--
				if m.cursorLeft < m.offsetLeft {
					m.offsetLeft--
				}
			} else if m.focus == "right" && m.cursorRight > 0 {
				m.cursorRight--
				if m.cursorRight < m.offsetRight {
					m.offsetRight--
				}
			} else if m.focus == "terminal" && m.historyIndex > 0 {
				m.historyIndex--
				m.termInput = m.history[m.historyIndex]
				m.cursorPos = len(m.termInput)
			}

		case "down":
			if m.focus == "left" && m.cursorLeft < len(m.leftFiles)-1 {
				m.cursorLeft++
				if m.cursorLeft >= m.offsetLeft+(m.height/2+4-2) {
					m.offsetLeft++
				}
			} else if m.focus == "right" && m.cursorRight < len(m.rightFiles)-1 {
				m.cursorRight++
				if m.cursorRight >= m.offsetRight+(m.height/2+4-2) {
					m.offsetRight++
				}
			} else if m.focus == "terminal" {
				if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
					m.termInput = m.history[m.historyIndex]
				} else {
					m.historyIndex = len(m.history)
					m.termInput = ""
				}
				m.cursorPos = len(m.termInput)
			}

		case "left":
			if m.focus == "terminal" && m.cursorPos > 0 {
				m.cursorPos--
			} else if m.focus == "left" {
				m.leftDir = parentDir(m.leftDir)
				m.leftFiles, _ = os.ReadDir(m.leftDir)
				m.cursorLeft, m.offsetLeft = 0, 0
			} else if m.focus == "right" {
				m.rightDir = parentDir(m.rightDir)
				m.rightFiles, _ = os.ReadDir(m.rightDir)
				m.cursorRight, m.offsetRight = 0, 0
			}

		case "right":
			if m.focus == "terminal" && m.cursorPos < len(m.termInput) {
				m.cursorPos++
			} else if m.focus == "left" {
				m = enterItem(m, true)
			} else if m.focus == "right" {
				m = enterItem(m, false)
			}

		case "backspace":
			if m.focus == "terminal" && m.cursorPos > 0 {
				before := m.termInput[:m.cursorPos-1]
				after := m.termInput[m.cursorPos:]
				m.termInput = before + after
				m.cursorPos--
			}

		case "enter":
			if m.focus == "terminal" {
				cmd := strings.TrimSpace(m.termInput)
				// фиксируем ввод
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput

				if cmd != "" {
					result := runCommand(cmd)
					if result != "" {
						for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
							m.termOutput = append(m.termOutput, line)
						}
					}
					m.history = append(m.history, cmd)
				}
				m.termInput = ""
				m.cursorPos = 0
				m.historyIndex = len(m.history)
				m.termOutput = append(m.termOutput, "$ ")
			}

		default:
			if m.focus == "terminal" && msg.Type == tea.KeyRunes {
				r := string(msg.Runes)
				before := m.termInput[:m.cursorPos]
				after := m.termInput[m.cursorPos:]
				m.termInput = before + r + after
				m.cursorPos += len(r)
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
	panelH := m.height/2 + 4

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, m.focus == "left", panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, m.focus == "right", panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	// терминал снизу
	termH := m.height - panelH - 2
	terminal := renderTerminal(m.termOutput, m.termInput, m.cursorPos, termH, m.width, m.focus == "terminal")

	return ui + "\n" + terminal
}

func renderPanel(path string, files []os.DirEntry, cursor, offset int, active bool, w, h int) string {
	if w <= 2 {
		w = 4
	}
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
		if runewidth.StringWidth(stripANSI(line)) > w-2 {
			runes := []rune(stripANSI(line))
			line = string(runes[:w-2])
		}
		pad := w - 2 - runewidth.StringWidth(stripANSI(line))
		if pad < 0 {
			pad = 0
		}
		b.WriteString(color + "|" + reset + line +
			strings.Repeat(" ", pad) +
			color + "|" + reset + "\n")
	}

	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

func renderTerminal(output []string, input string, cursorPos int, h, w int, active bool) string {
	if h < 3 {
		h = 3
	}
	border := "-"
	color := gray
	if active {
		border = "="
		color = green
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset + "\n")

	start := 0
	if len(output) > h-3 {
		start = len(output) - (h - 3)
	}
	visible := output[start:]

	for _, line := range visible {
		clean := stripANSI(line)
		if runewidth.StringWidth(clean) > w-2 {
			runes := []rune(clean)
			clean = string(runes[:w-2])
		}
		pad := w - 2 - runewidth.StringWidth(clean)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(color + "|" + reset + clean +
			strings.Repeat(" ", pad) +
			color + "|" + reset + "\n")
	}

	// строка ввода
	cursorLine := "$ " + input
	if active {
		pos := 2 + runewidth.StringWidth(input[:cursorPos])
		cursorLine = "$ " + input[:cursorPos] + "_" + input[cursorPos:]
		if pos >= w-1 {
			pos = w - 2
		}
	}
	if runewidth.StringWidth(cursorLine) > w-2 {
		runes := []rune(cursorLine)
		cursorLine = string(runes[:w-2])
	}
	pad := w - 2 - runewidth.StringWidth(stripANSI(cursorLine))
	b.WriteString(color + "|" + reset + cursorLine +
		strings.Repeat(" ", pad) +
		color + "|" + reset + "\n")

	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

func runCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	c := exec.Command(parts[0], parts[1:]...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out

	err := c.Run()
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	return out.String()
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
		m.termOutput = append(m.termOutput, fmt.Sprintf("Открыт файл: %s", path))
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
