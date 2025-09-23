package main

import (
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

	cursorLeft  int
	cursorRight int
	offsetLeft  int
	offsetRight int
	activePane  string

	termActive   bool
	termInput    string
	termOutput   []string
	termDir      string
	termHistory  []string
	termHistIdx  int
	termScroll   int
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
		termDir:     wd,
		termOutput:  []string{"Терминал готов"},
		termHistory: []string{},
		termHistIdx: -1,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.termActive {
			switch msg.String() {
			case "ctrl+q":
				return m, tea.Quit
			case "tab":
				m.termActive = false
			case "enter":
				input := strings.TrimSpace(m.termInput)
				if input != "" {
					if len(m.termHistory) == 0 || m.termHistory[len(m.termHistory)-1] != input {
						m.termHistory = append(m.termHistory, input)
					}
					m.termHistIdx = len(m.termHistory)
					m.executeCommand(input)
				}
				m.termInput = ""
			case "backspace":
				if len(m.termInput) > 0 {
					m.termInput = m.termInput[:len(m.termInput)-1]
				}
			case "up":
				if len(m.termHistory) > 0 && m.termHistIdx > 0 {
					m.termHistIdx--
					m.termInput = m.termHistory[m.termHistIdx]
				}
			case "down":
				if m.termHistIdx < len(m.termHistory)-1 {
					m.termHistIdx++
					m.termInput = m.termHistory[m.termHistIdx]
				} else {
					m.termHistIdx = len(m.termHistory)
					m.termInput = ""
				}
			case "pgup":
				if m.termScroll < len(m.termOutput)-1 {
					m.termScroll += 1
				}
			case "pgdown":
				if m.termScroll > 0 {
					m.termScroll -= 1
				}
			case " ":
				m.termInput += " "
			default:
				if msg.Type == tea.KeyRunes {
					m.termInput += string(msg.Runes)
				}
			}
		} else {
			switch msg.String() {
			case "ctrl+q":
				return m, tea.Quit
			case "tab":
				m.termActive = true
			case "alt+left":
				m.activePane = "left"
			case "alt+right":
				m.activePane = "right"
			case "left":
				if m.activePane == "left" {
					m.leftDir = parentDir(m.leftDir)
					m.leftFiles, _ = os.ReadDir(m.leftDir)
					m.cursorLeft, m.offsetLeft = 0, 0
				} else {
					m.rightDir = parentDir(m.rightDir)
					m.rightFiles, _ = os.ReadDir(m.rightDir)
					m.cursorRight, m.offsetRight = 0, 0
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
					if m.cursorLeft < m.offsetLeft {
						m.offsetLeft--
					}
				} else if m.activePane == "right" && m.cursorRight > 0 {
					m.cursorRight--
					if m.cursorRight < m.offsetRight {
						m.offsetRight--
					}
				}
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
	panelH := m.height/2 + 4

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, !m.termActive && m.activePane == "left", panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, !m.termActive && m.activePane == "right", panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	termLines := m.renderTerminal()
	return ui + "\n" + strings.Join(termLines, "\n")
}

func renderPanel(path string, files []os.DirEntry, cursor, offset int, active bool, w, h int) string {
	borderChar := "-"
	color := gray
	if active {
		borderChar = "="
		color = green
	}

	repeat := w - 2
	if repeat < 0 {
		repeat = 0
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(borderChar, repeat) + "+" + reset + "\n")

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
			line = runewidth.Truncate(stripANSI(line), w-2, "")
		}
		b.WriteString(color + "|" + reset + line +
			strings.Repeat(" ", max(0, w-2-runewidth.StringWidth(stripANSI(line)))) +
			color + "|" + reset + "\n")
	}

	b.WriteString(color + "+" + strings.Repeat(borderChar, repeat) + "+" + reset)
	return b.String()
}

func (m model) renderTerminal() []string {
	h := m.height/2 - 4
	borderChar := "-"
	color := gray
	if m.termActive {
		borderChar = "="
		color = green
	}

	lines := []string{color + "+" + strings.Repeat(borderChar, max(0, m.width-2)) + "+" + reset}

	start := max(0, len(m.termOutput)-h+m.termScroll)
	end := max(0, len(m.termOutput)-m.termScroll)
	if start > end {
		start = end
	}
	visible := m.termOutput[start:end]

	for _, out := range visible {
		if runewidth.StringWidth(out) > m.width-2 {
			out = runewidth.Truncate(out, m.width-2, "")
		}
		lines = append(lines, color+"|"+reset+out+
			strings.Repeat(" ", max(0, m.width-2-runewidth.StringWidth(out)))+
			color+"|"+reset)
	}

	input := "$ " + m.termInput
	cursor := yellow + "█" + reset
	if runewidth.StringWidth(input) < m.width-2 {
		input = input + cursor
	} else {
		input = runewidth.Truncate(input, m.width-3, "") + cursor
	}
	lines = append(lines, color+"|"+reset+input+
		strings.Repeat(" ", max(0, m.width-2-runewidth.StringWidth(input)))+
		color+"|"+reset)

	lines = append(lines, color+"+"+strings.Repeat(borderChar, max(0, m.width-2))+"+"+reset)

	return lines
}

func (m *model) executeCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "cd":
		if len(args) > 0 {
			newPath := args[0]
			if !filepath.IsAbs(newPath) {
				newPath = filepath.Join(m.termDir, newPath)
			}
			if fi, err := os.Stat(newPath); err == nil && fi.IsDir() {
				m.termDir = newPath
			} else {
				m.termOutput = append(m.termOutput, "Нет такого каталога: "+newPath)
			}
		}
	case "pwd":
		m.termOutput = append(m.termOutput, m.termDir)
	case "ls":
		files, err := os.ReadDir(m.termDir)
		if err != nil {
			m.termOutput = append(m.termOutput, "Ошибка: "+err.Error())
			return
		}
		var names []string
		for _, f := range files {
			name := f.Name()
			if f.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		m.termOutput = append(m.termOutput, strings.Join(names, "  "))
	default:
		out, err := exec.Command(cmd, args...).CombinedOutput()
		if err != nil {
			m.termOutput = append(m.termOutput, "Ошибка: "+err.Error())
		}
		if len(out) > 0 {
			m.termOutput = append(m.termOutput, strings.Split(strings.TrimSpace(string(out)), "\n")...)
		}
	}
}

func enterItem(m model, left bool) model {
	var dir string
	var files []os.DirEntry
	var cursor int

	if left {
		dir, files, cursor = m.leftDir, m.leftFiles, m.cursorLeft
	} else {
		dir, files, cursor = m.rightDir, m.rightFiles, m.cursorRight
	}

	if cursor >= len(files) {
		return m
	}

	item := files[cursor]
	path := filepath.Join(dir, item.Name())
	if item.IsDir() {
		newFiles, _ := os.ReadDir(path)
		if left {
			m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft = path, newFiles, 0, 0
		} else {
			m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight = path, newFiles, 0, 0
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
	for _, code := range []string{reset, green, gray, yellow} {
		s = strings.ReplaceAll(s, code, "")
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Ошибка запуска:", err)
		os.Exit(1)
	}
}
