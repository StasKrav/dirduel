package main

import (
	"fmt"
	"io"
	"os"
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
	termScrollPos int
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
			}

		// управление панелями
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

		// навигация в панелях
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
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput
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
			} else if m.focus == "terminal" && m.historyIndex < len(m.history)-1 {
				m.historyIndex++
				m.termInput = m.history[m.historyIndex]
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput
			} else if m.focus == "terminal" {
				m.historyIndex = len(m.history)
				m.termInput = ""
				m.termOutput[len(m.termOutput)-1] = "$ "
			}

		case "left":
			if m.focus == "left" {
				m.leftDir = parentDir(m.leftDir)
				m.leftFiles, _ = os.ReadDir(m.leftDir)
				m.cursorLeft, m.offsetLeft = 0, 0
			} else if m.focus == "right" {
				m.rightDir = parentDir(m.rightDir)
				m.rightFiles, _ = os.ReadDir(m.rightDir)
				m.cursorRight, m.offsetRight = 0, 0
			}

		case "right":
			if m.focus == "left" {
				m = enterItem(m, true)
			} else if m.focus == "right" {
				m = enterItem(m, false)
			}

		// ввод в терминале
		case "backspace":
			if m.focus == "terminal" && len(m.termInput) > 0 {
				m.termInput = m.termInput[:len(m.termInput)-1]
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput
			}
		case "enter":
			if m.focus == "terminal" {
				cmd := strings.TrimSpace(m.termInput)
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput
				if cmd != "" {
					m.termOutput = append(m.termOutput, runCommand(cmd))
					m.history = append(m.history, cmd)
				}
				m.termInput = ""
				m.historyIndex = len(m.history)
				m.termOutput = append(m.termOutput, "$ ")
			}
		default:
			// ✅ Поддержка кириллицы и любых Unicode-символов
			if m.focus == "terminal" && len(msg.Runes) > 0 {
				m.termInput += string(msg.Runes)
				m.termOutput[len(m.termOutput)-1] = "$ " + m.termInput
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
	terminal := renderTerminal(m.termOutput, termH, m.width, m.focus == "terminal")

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
			lineRunes := []rune(stripANSI(line))
			line = string(lineRunes[:w-2])
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

func renderTerminal(output []string, h, w int, active bool) string {
	if h < 3 {
		h = 3
	}
	border := "-"
	color := gray
	if active {
		border = "="
		color = green
	}

	rep := func(s string, n int) string {
		if n <= 0 {
			return ""
		}
		return strings.Repeat(s, n)
	}

	var b strings.Builder
	b.WriteString(color + "+" + rep(border, w-2) + "+" + reset + "\n")

	start := 0
	if len(output) > h-2 {
		start = len(output) - (h - 2)
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

	b.WriteString(color + "+" + rep(border, w-2) + "+" + reset)
	return b.String()
}

func runCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	switch parts[0] {
	case "ls":
		wd, _ := os.Getwd()
		files, err := os.ReadDir(wd)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		names := []string{}
		for _, f := range files {
			name := f.Name()
			if f.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		return strings.Join(names, "  ")

	case "pwd":
		wd, _ := os.Getwd()
		return wd

	case "cd":
		if len(parts) < 2 {
			return "Использование: cd <dir>"
		}
		err := os.Chdir(parts[1])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "cat":
		if len(parts) < 2 {
			return "Использование: cat <file>"
		}
		data, err := os.ReadFile(parts[1])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return string(data)

	case "mkdir":
		if len(parts) < 2 {
			return "Использование: mkdir <dir>"
		}
		err := os.Mkdir(parts[1], 0755)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "touch":
		if len(parts) < 2 {
			return "Использование: touch <file>"
		}
		_, err := os.Create(parts[1])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "rm":
		if len(parts) < 2 {
			return "Использование: rm <file>"
		}
		err := os.Remove(parts[1])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "cp":
		if len(parts) < 3 {
			return "Использование: cp <src> <dst>"
		}
		src, dst := parts[1], parts[2]
		in, err := os.Open(src)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		defer in.Close()
		out, err := os.Create(dst)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "mv":
		if len(parts) < 3 {
			return "Использование: mv <src> <dst>"
		}
		err := os.Rename(parts[1], parts[2])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "echo":
		return strings.Join(parts[1:], " ")

	default:
		return "Неизвестная команда: " + parts[0]
	}
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
