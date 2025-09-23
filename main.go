package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	termInput    []rune
	termOutput   []string
	termScroll   int
	termHistory  []string
	historyIndex int
}

func initialModel() model {
	wd, _ := os.Getwd()
	files, _ := os.ReadDir(wd)
	return model{
		activePane:   "left",
		leftDir:      wd,
		rightDir:     wd,
		leftFiles:    files,
		rightFiles:   files,
		termOutput:   []string{"Терминал готов. Введите `help` для списка команд."},
		termHistory:  []string{},
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
		if m.termActive {
			switch msg.String() {
			case "tab":
				m.termActive = false
			case "enter":
				cmd := string(m.termInput)
				m.termOutput = append(m.termOutput, "$ "+cmd)
				m.executeCommand(cmd)
				if cmd != "" {
					m.termHistory = append(m.termHistory, cmd)
					m.historyIndex = len(m.termHistory)
				}
				m.termInput = nil
			case "backspace":
				if len(m.termInput) > 0 {
					m.termInput = m.termInput[:len(m.termInput)-1]
				}
			case "up":
				if len(m.termHistory) > 0 && m.historyIndex > 0 {
					m.historyIndex--
					m.termInput = []rune(m.termHistory[m.historyIndex])
				}
			case "down":
				if len(m.termHistory) > 0 && m.historyIndex < len(m.termHistory)-1 {
					m.historyIndex++
					m.termInput = []rune(m.termHistory[m.historyIndex])
				} else {
					m.historyIndex = len(m.termHistory)
					m.termInput = nil
				}
			case "pageup":
				if m.termScroll < len(m.termOutput)-1 {
					m.termScroll++
				}
			case "pagedown":
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
			case "tab":
				m.termActive = true
			}
		}
	}
	return m, nil
}

func (m *model) executeCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "help":
		m.termOutput = append(m.termOutput,
			"Доступные команды:",
			"  help   - список команд",
			"  ls     - список файлов",
			"  pwd    - текущая директория",
			"  cd     - сменить директорию",
			"  cat    - показать файл",
			"  date   - текущая дата/время",
			"  clear  - очистить экран",
			"  echo   - вывести текст",
			"  exit   - выйти")
	case "pwd":
		m.termOutput = append(m.termOutput, m.leftDir)
	case "ls":
		files, _ := os.ReadDir(m.leftDir)
		var names []string
		for _, f := range files {
			if f.IsDir() {
				names = append(names, f.Name()+"/")
			} else {
				names = append(names, f.Name())
			}
		}
		m.termOutput = append(m.termOutput, strings.Join(names, "  "))
	case "cd":
		if len(parts) < 2 {
			m.termOutput = append(m.termOutput, "Укажите директорию")
			return
		}
		newPath := filepath.Join(m.leftDir, parts[1])
		if st, err := os.Stat(newPath); err == nil && st.IsDir() {
			m.leftDir = newPath
			m.leftFiles, _ = os.ReadDir(m.leftDir)
			m.cursorLeft, m.offsetLeft = 0, 0
			m.termOutput = append(m.termOutput, "Текущая директория: "+m.leftDir)
		} else {
			m.termOutput = append(m.termOutput, "Нет такой директории")
		}
	case "cat":
		if len(parts) < 2 {
			m.termOutput = append(m.termOutput, "Укажите файл")
			return
		}
		filePath := filepath.Join(m.leftDir, parts[1])
		data, err := os.ReadFile(filePath)
		if err != nil {
			m.termOutput = append(m.termOutput, "Ошибка: "+err.Error())
			return
		}
		lines := strings.Split(string(data), "\n")
		m.termOutput = append(m.termOutput, lines...)
	case "date":
		m.termOutput = append(m.termOutput, time.Now().Format(time.RFC1123))
	case "clear":
		m.termOutput = nil
	case "echo":
		m.termOutput = append(m.termOutput, strings.Join(parts[1:], " "))
	case "exit":
		m.termOutput = append(m.termOutput, "Выход...")
		os.Exit(0)
	default:
		m.termOutput = append(m.termOutput, "Неизвестная команда: "+parts[0])
	}
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Загрузка..."
	}

	panelW := m.width/2 - 2
	panelH := m.height/2 + 4

	left := renderPanel(m.leftDir, m.leftFiles, m.cursorLeft, m.offsetLeft, m.activePane == "left", panelW, panelH)
	right := renderPanel(m.rightDir, m.rightFiles, m.cursorRight, m.offsetRight, m.activePane == "right", panelW, panelH)

	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	var combined []string
	for i := 0; i < len(linesL); i++ {
		combined = append(combined, linesL[i]+"  "+linesR[i])
	}
	ui := strings.Join(combined, "\n")

	// Рисуем терминал
	termH := m.height - panelH - 2
	var termLines []string
	start := 0
	if len(m.termOutput) > termH-2 {
		start = len(m.termOutput) - (termH - 2) - m.termScroll
		if start < 0 {
			start = 0
		}
	}
	visible := m.termOutput[start:]
	if len(visible) > termH-2 {
		visible = visible[:termH-2]
	}
	border := gray
	if m.termActive {
		border = green
	}
	termLines = append(termLines, border+"+"+strings.Repeat("-", m.width-2)+"+"+reset)
	for _, l := range visible {
		line := fitStringToWidth(l, m.width-2)
		termLines = append(termLines, border+"|"+reset+line+border+"|"+reset)
	}
	// строка ввода
	input := string(m.termInput)
	input = fitStringToWidth(input, m.width-2)
	termLines = append(termLines, border+"|"+reset+input+border+"|"+reset)
	termLines = append(termLines, border+"+"+strings.Repeat("-", m.width-2)+"+"+reset)

	return ui + "\n" + strings.Join(termLines, "\n")
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
		b.WriteString(color + "|" + reset + line +
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

func fitStringToWidth(s string, width int) string {
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

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Ошибка запуска:", err)
		os.Exit(1)
	}
}
