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

	cursorLeft   int
	cursorRight  int
	offsetLeft   int
	offsetRight  int
	activePane   string

	// терминал
	termActive     bool
	termInput      []rune
	termOutput     []string
	termHistory    []string
	termHistIndex  int
	termScroll     int
}

func initialModel() model {
	wd, _ := os.Getwd()
	files, _ := os.ReadDir(wd)
	return model{
		activePane:    "left",
		leftDir:       wd,
		rightDir:      wd,
		leftFiles:     files,
		rightFiles:    files,
		termOutput:    []string{"Терминал готов"},
		termHistory:   []string{},
		termHistIndex: -1,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.termActive {
			switch msg.String() {
			case "tab":
				m.termActive = false
			case "enter":
				input := string(m.termInput)
				m.termHistory = append(m.termHistory, input)
				m.termHistIndex = -1
				m.termOutput = append(m.termOutput, "> "+input)
				m.termInput = nil
				return m, execCommandCmd(input)
			case "backspace":
				if len(m.termInput) > 0 {
					m.termInput = m.termInput[:len(m.termInput)-1]
				}
			case "up":
				if len(m.termHistory) > 0 {
					if m.termHistIndex == -1 {
						m.termHistIndex = len(m.termHistory) - 1
					} else if m.termHistIndex > 0 {
						m.termHistIndex--
					}
					m.termInput = []rune(m.termHistory[m.termHistIndex])
				}
			case "down":
				if m.termHistIndex >= 0 {
					if m.termHistIndex < len(m.termHistory)-1 {
						m.termHistIndex++
						m.termInput = []rune(m.termHistory[m.termHistIndex])
					} else {
						m.termHistIndex = -1
						m.termInput = nil
					}
				}
			case "pgup":
				if m.termScroll < len(m.termOutput)-1 {
					m.termScroll++
				}
			case "pgdown":
				if m.termScroll > 0 {
					m.termScroll--
				}
			case " ":
				m.termInput = append(m.termInput, ' ')
			default:
				if msg.Type == tea.KeyRunes {
					m.termInput = append(m.termInput, msg.Runes...)
				}
			}
		} else {
			switch msg.String() {
			case "ctrl+q":
				return m, tea.Quit
			case "tab":
				m.termActive = true
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
			case "alt+left":
				m.activePane = "left"
			case "alt+right":
				m.activePane = "right"
			}
		}
	case commandOutputMsg:
		lines := strings.Split(string(msg), "\n")
		m.termOutput = append(m.termOutput, lines...)
		if len(m.termOutput) > 1000 {
			m.termOutput = m.termOutput[len(m.termOutput)-1000:]
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

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, m.activePane == "left" && !m.termActive, panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, m.activePane == "right" && !m.termActive, panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	// терминал
	termH := m.height - panelH - 2
	ui += "\n" + renderTerminal(m.termOutput, m.termInput, m.termScroll, m.termActive, m.width-2, termH)

	return ui
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
		line = fitStringToWidth(stripANSI(line), w-2)
		b.WriteString(color + "|" + reset + line + color + "|" + reset + "\n")
	}

	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

func renderTerminal(output []string, input []rune, scroll int, active bool, w, h int) string {
	border := "-"
	color := gray
	if active {
		border = "="
		color = green
	}

	var b strings.Builder
	b.WriteString(color + "+" + strings.Repeat(border, w) + "+" + reset + "\n")

	start := 0
	if len(output)-scroll > h-2 {
		start = len(output) - scroll - (h - 2)
	}
	visible := output[start : len(output)-scroll]

	for i := 0; i < h-2; i++ {
		var line string
		if i < len(visible) {
			line = visible[i]
		}
		line = fitStringToWidth(line, w)
		b.WriteString(color + "|" + reset + line + color + "|" + reset + "\n")
	}

	prompt := "> " + string(input)
	if active {
		prompt += "_"
	}
	prompt = fitStringToWidth(prompt, w)
	b.WriteString(color + "|" + reset + prompt + color + "|" + reset + "\n")

	b.WriteString(color + "+" + strings.Repeat(border, w) + "+" + reset)
	return b.String()
}

func fitStringToWidth(s string, width int) string {
	s = stripANSI(s)
	out := ""
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > width {
			break
		}
		out += string(r)
		w += rw
	}
	if w < width {
		out += strings.Repeat(" ", width-w)
	}
	return out
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

// --- терминал эмуляция ---

type commandOutputMsg string

func execCommandCmd(input string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("bash", "-c", input)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			return commandOutputMsg(string(out) + "\nОшибка: " + err.Error())
		}
		return commandOutputMsg(string(out))
	}
}

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Ошибка запуска:", err)
		os.Exit(1)
	}
}
