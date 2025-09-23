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

	termActive  bool
	termInput   string
	termOutput  []string
	termDir     string
	termHistory []string
	termHistIdx int
	termScroll  int
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stripANSI(s string) string {
	s = strings.ReplaceAll(s, reset, "")
	s = strings.ReplaceAll(s, green, "")
	s = strings.ReplaceAll(s, gray, "")
	s = strings.ReplaceAll(s, yellow, "")
	return s
}

/* ---------------- Rendering ---------------- */

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

	title := fmt.Sprintf(" %s", filepath.Base(path))
	if title == " " {
		title = " /"
	}
	if runewidth.StringWidth(title) > repeat {
		title = runewidth.Truncate(title, repeat, "")
	}
	b.WriteString(color + "|" + reset + title + strings.Repeat(" ", max(0, repeat-runewidth.StringWidth(title))) + color + "|" + reset + "\n")

	visibleFiles := files
	if len(files) > h-3 {
		end := offset + (h - 3)
		if end > len(files) {
			end = len(files)
		}
		visibleFiles = files[offset:end]
	}

	for i := 0; i < h-3; i++ {
		var line string
		if i < len(visibleFiles) {
			idx := offset + i
			name := visibleFiles[i].Name()
			if visibleFiles[i].IsDir() {
				name += "/"
			}
			line = " " + name
			if idx == cursor && active {
				line = yellow + ">" + line + reset
			}
		}
		disp := stripANSI(line)
		if runewidth.StringWidth(disp) > repeat {
			disp = runewidth.Truncate(disp, repeat, "")
		}
		pad := max(0, repeat-runewidth.StringWidth(disp))
		b.WriteString(color + "|" + reset + disp + strings.Repeat(" ", pad) + color + "|" + reset + "\n")
	}

	b.WriteString(color + "+" + strings.Repeat(borderChar, repeat) + "+" + reset)
	return b.String()
}

func (m model) renderTerminal() []string {
	panelH := m.height/2 + 4
	termH := m.height - panelH - 1
	if termH < 3 {
		termH = 3
	}

	borderChar := "-"
	color := gray
	if m.termActive {
		borderChar = "="
		color = green
	}

	repeat := m.width - 2
	if repeat < 0 {
		repeat = 0
	}

	lines := []string{color + "+" + strings.Repeat(borderChar, repeat) + "+" + reset}

	available := termH - 2
	start := max(0, len(m.termOutput)-available-m.termScroll)
	end := max(0, len(m.termOutput)-m.termScroll)
	if start > end {
		start = end
	}
	visible := m.termOutput[start:end]

	for _, out := range visible {
		disp := stripANSI(out)
		if runewidth.StringWidth(disp) > repeat {
			disp = runewidth.Truncate(disp, repeat, "")
		}
		pad := max(0, repeat-runewidth.StringWidth(disp))
		lines = append(lines, color+"|"+reset+disp+strings.Repeat(" ", pad)+color+"|"+reset)
	}

	prompt := "$ " + m.termInput
	if runewidth.StringWidth(stripANSI(prompt)) >= repeat {
		trunc := runewidth.Truncate(stripANSI(prompt), max(0, repeat-1), "")
		display := trunc + yellow + "█" + reset
		lines = append(lines, color+"|"+reset+display+strings.Repeat(" ", max(0, repeat-runewidth.StringWidth(stripANSI(trunc))))+color+"|"+reset)
	} else {
		display := prompt + yellow + "█" + reset
		lines = append(lines, color+"|"+reset+display+strings.Repeat(" ", max(0, repeat-runewidth.StringWidth(stripANSI(prompt))))+color+"|"+reset)
	}

	lines = append(lines, color+"+"+strings.Repeat(borderChar, repeat)+"+"+reset)
	return lines
}

/* ---------------- Commands ---------------- */

func (m *model) executeCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "cd":
		if len(args) == 0 {
			home, err := os.UserHomeDir()
			if err == nil {
				m.termDir = home
			}
			return
		}
		target := args[0]
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.termDir, target)
		}
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			m.termOutput = append(m.termOutput, "Нет такой директории: "+args[0])
			return
		}
		m.termDir = target

	case "pwd":
		m.termOutput = append(m.termOutput, m.termDir)

	case "ls":
		dir := m.termDir
		if len(args) > 0 {
			d := args[0]
			if !filepath.IsAbs(d) {
				d = filepath.Join(m.termDir, d)
			}
			dir = d
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			m.termOutput = append(m.termOutput, "Ошибка: "+err.Error())
			return
		}
		var names []string
		for _, e := range ents {
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		m.termOutput = append(m.termOutput, strings.Join(names, "  "))

	case "clear":
		m.termOutput = []string{}

	default:
		c := exec.Command(cmd, args...)
		c.Dir = m.termDir
		out, err := c.CombinedOutput()
		if err != nil {
			if len(out) > 0 {
				lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
				m.termOutput = append(m.termOutput, lines...)
			}
			m.termOutput = append(m.termOutput, "Ошибка: "+err.Error())
			return
		}
		if len(out) > 0 {
			lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
			m.termOutput = append(m.termOutput, lines...)
		}
	}
}

/* ---------------- Update / View ---------------- */

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+q" {
			return m, tea.Quit
		}
		if key == "tab" {
			m.termActive = !m.termActive
			return m, nil
		}

		if m.termActive {
			switch key {
			case "enter":
				m.termOutput = append(m.termOutput, "$ "+m.termInput)
				inputTrim := strings.TrimSpace(m.termInput)
				if inputTrim != "" {
					if len(m.termHistory) == 0 || m.termHistory[len(m.termHistory)-1] != m.termInput {
						m.termHistory = append(m.termHistory, m.termInput)
					}
					m.termHistIdx = len(m.termHistory)
					m.executeCommand(m.termInput)
				}
				m.termInput = ""
				m.termScroll = 0
			case "backspace":
				if len(m.termInput) > 0 {
					runes := []rune(m.termInput)
					m.termInput = string(runes[:len(runes)-1])
				}
			case "up":
				if len(m.termHistory) > 0 {
					if m.termHistIdx == -1 {
						m.termHistIdx = len(m.termHistory) - 1
					} else if m.termHistIdx > 0 {
						m.termHistIdx--
					}
					if m.termHistIdx >= 0 && m.termHistIdx < len(m.termHistory) {
						m.termInput = m.termHistory[m.termHistIdx]
					}
				}
			case "down":
				if m.termHistIdx != -1 {
					if m.termHistIdx < len(m.termHistory)-1 {
						m.termHistIdx++
						m.termInput = m.termHistory[m.termHistIdx]
					} else {
						m.termHistIdx = -1
						m.termInput = ""
					}
				}
			case "pgup", "pageup":
				if m.termScroll < max(0, len(m.termOutput)-1) {
					m.termScroll++
				}
			case "pgdown", "pagedown":
				if m.termScroll > 0 {
					m.termScroll--
				}
			case " ":
				m.termInput += " "
			default:
				if msg.Type == tea.KeyRunes {
					m.termInput += string(msg.Runes)
				}
			}
		} else {
			switch key {
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

	leftS := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, !m.termActive && m.activePane == "left", panelW, panelH)
	rightS := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, !m.termActive && m.activePane == "right", panelW, panelH)

	linesL := strings.Split(leftS, "\n")
	linesR := strings.Split(rightS, "\n")
	maxLines := max(len(linesL), len(linesR))
	for len(linesL) < maxLines {
		linesL = append(linesL, strings.Repeat(" ", panelW))
	}
	for len(linesR) < maxLines {
		linesR = append(linesR, strings.Repeat(" ", panelW))
	}

	var combined []string
	for i := 0; i < maxLines; i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	termLines := m.renderTerminal()
	return ui + "\n" + strings.Join(termLines, "\n")
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

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Ошибка запуска:", err)
		os.Exit(1)
	}
}
