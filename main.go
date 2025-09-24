package main

import (
	"fmt"
	"io"
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

	leftDir, rightDir string
	leftFiles, rightFiles []os.DirEntry

	cursorLeft, cursorRight int
	offsetLeft, offsetRight int
	activePane, focus string // "left", "right", "terminal"

	termInput     []rune
	termCursorPos int
	termOutput    []string
	history       []string
	historyIndex  int
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
		termOutput:   []string{},
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
		switch msg.Type {

		// global
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyTab:
			if m.focus == "terminal" {
				m.focus = m.activePane
			} else {
				m.focus = "terminal"
				m.termCursorPos = len(m.termInput)
			}

		// Left / Right with Alt = switch active panel or normal navigation
		case tea.KeyLeft:
			if msg.Alt {
				if m.focus != "terminal" {
					m.activePane = "left"
					m.focus = "left"
				}
				break
			}
			// normal Left
			if m.focus == "terminal" {
				if m.termCursorPos > 0 {
					m.termCursorPos--
				}
			} else if m.focus == "left" {
				m.leftDir = parentDir(m.leftDir)
				m.leftFiles, _ = os.ReadDir(m.leftDir)
				m.cursorLeft, m.offsetLeft = 0, 0
			} else if m.focus == "right" {
				m.rightDir = parentDir(m.rightDir)
				m.rightFiles, _ = os.ReadDir(m.rightDir)
				m.cursorRight, m.offsetRight = 0, 0
			}

		case tea.KeyRight:
			if msg.Alt {
				if m.focus != "terminal" {
					m.activePane = "right"
					m.focus = "right"
				}
				break
			}
			if m.focus == "terminal" {
				if m.termCursorPos < len(m.termInput) {
					m.termCursorPos++
				}
			} else if m.focus == "left" {
				m = enterItem(m, true)
			} else if m.focus == "right" {
				m = enterItem(m, false)
			}

		// Up / Down (panels navigation or terminal history)
		case tea.KeyUp:
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
				m.termInput = []rune(m.history[m.historyIndex])
				m.termCursorPos = len(m.termInput)
			}

		case tea.KeyDown:
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
				m.termInput = []rune(m.history[m.historyIndex])
				m.termCursorPos = len(m.termInput)
			} else if m.focus == "terminal" {
				m.historyIndex = len(m.history)
				m.termInput = []rune{}
				m.termCursorPos = 0
			}

		// editing / typing
		case tea.KeyBackspace:
			if m.focus == "terminal" && m.termCursorPos > 0 {
				r := m.termInput
				r = append(r[:m.termCursorPos-1], r[m.termCursorPos:]...)
				m.termInput = r
				m.termCursorPos--
			}

		case tea.KeyDelete:
			if m.focus == "terminal" && m.termCursorPos < len(m.termInput) {
				r := m.termInput
				r = append(r[:m.termCursorPos], r[m.termCursorPos+1:]...)
				m.termInput = r
			}

		case tea.KeySpace:
			if m.focus == "terminal" {
				before := append([]rune{}, m.termInput[:m.termCursorPos]...)
				after := append([]rune{}, m.termInput[m.termCursorPos:]...)
				m.termInput = append(before, append([]rune{' '}, after...)...)
				m.termCursorPos++
			}

		case tea.KeyRunes:
			if m.focus == "terminal" {
				r := msg.Runes // []rune (can be multirune for composed input)
				before := append([]rune{}, m.termInput[:m.termCursorPos]...)
				after := append([]rune{}, m.termInput[m.termCursorPos:]...)
				new := append(before, append(r, after...)...)
				m.termInput = new
				m.termCursorPos += len(r)
			}

		case tea.KeyEnter:
			if m.focus == "terminal" {
				cmd := strings.TrimSpace(string(m.termInput))
				// показываем введённую команду в выводе терминала
				if cmd != "" {
					m.termOutput = append(m.termOutput, "$ "+cmd)

					result := runCommand(cmd)
					if result != "" {
						for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
							m.termOutput = append(m.termOutput, line)
						}
					}
					// add to history
					m.history = append(m.history, cmd)
				}
				// reset input
				m.termInput = []rune{}
				m.termCursorPos = 0
				m.historyIndex = len(m.history)
			} else if m.focus == "left" {
				m = enterItem(m, true)
			} else if m.focus == "right" {
				m = enterItem(m, false)
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

	// combine panels line by line safely
	ll := strings.Split(left, "\n")
	rr := strings.Split(right, "\n")
	maxLines := max(len(ll), len(rr))
	var combined []string
	for i := 0; i < maxLines; i++ {
		leftLine := ""
		rightLine := ""
		if i < len(ll) {
			leftLine = ll[i]
		}
		if i < len(rr) {
			rightLine = rr[i]
		}
		combined = append(combined, leftLine+"  "+rightLine)
	}
	ui := strings.Join(combined, "\n")

	// terminal below
	termH := m.height - panelH - 2
	terminal := renderTerminal(m.termOutput, m.termInput, m.termCursorPos, termH, m.width, m.focus == "terminal")

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

	visible := files
	if len(files) > h-2 {
		end := offset + (h - 2)
		if end > len(files) {
			end = len(files)
		}
		visible = files[offset:end]
	}

	for i := 0; i < h-2; i++ {
		var line string
		if i < len(visible) {
			idx := offset + i
			name := visible[i].Name()
			if visible[i].IsDir() {
				name += "/"
			}
			line = fmt.Sprintf(" %s", name)
			if idx == cursor {
				line = yellow + ">" + line + reset
			}
		}
		clean := stripANSI(line)
		if runewidth.StringWidth(clean) > w-2 {
			line = truncateToWidth(clean, w-2)
		}
		pad := w - 2 - runewidth.StringWidth(stripANSI(line))
		if pad < 0 {
			pad = 0
		}
		b.WriteString(color + "|" + reset + line + strings.Repeat(" ", pad) + color + "|" + reset + "\n")
	}

	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

func renderTerminal(output []string, input []rune, cursor int, h, w int, active bool) string {
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

	// visible output lines (reserve one line for input)
	start := 0
	if len(output) > h-2 {
		start = len(output) - (h - 2)
	}
	visible := output[start:]

	for _, line := range visible {
		clean := stripANSI(line)
		if runewidth.StringWidth(clean) > w-2 {
			clean = truncateToWidth(clean, w-2)
		}
		pad := w - 2 - runewidth.StringWidth(clean)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(color + "|" + reset + clean + strings.Repeat(" ", pad) + color + "|" + reset + "\n")
	}

	// input line with cursor
	left := string(input[:min(cursor, len(input))])
	right := string(input[min(cursor, len(input)):])
	cursorLine := "$ " + left + "_" + right
	if runewidth.StringWidth(cursorLine) > w-2 {
		cursorLine = truncateFromRight(cursorLine, w-2)
	}
	pad := w - 2 - runewidth.StringWidth(stripANSI(cursorLine))
	if pad < 0 {
		pad = 0
	}
	b.WriteString(color + "|" + reset + cursorLine + strings.Repeat(" ", pad) + color + "|" + reset + "\n")

	b.WriteString(color + "+" + strings.Repeat(border, w-2) + "+" + reset)
	return b.String()
}

// runCommand отвечает за выполнение команд в терминале.
// Здесь добавлены: расширенный ls (с флагами -a, -l),
// clear (очистка терминала), ping (проверка доступности адреса),
// help (список всех поддерживаемых команд).
func runCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	switch parts[0] {

	// Новый ls с поддержкой флагов -a и -l
	case "ls":
		showAll := false // -a
		long := false    // -l
		args := []string{}

		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "-") {
				if strings.Contains(p, "a") {
					showAll = true
				}
				if strings.Contains(p, "l") {
					long = true
				}
			} else {
				args = append(args, p)
			}
		}

		wd := ""
		if len(args) > 0 {
			wd = args[0]
		} else {
			wd, _ = os.Getwd()
		}

		files, err := os.ReadDir(wd)
		if err != nil {
			return "Ошибка: " + err.Error()
		}

		var lines []string
		for _, f := range files {
			name := f.Name()
			if !showAll && strings.HasPrefix(name, ".") {
				continue
			}
			if f.IsDir() {
				name += "/"
			}
			if long {
				info, err := f.Info()
				if err != nil {
					lines = append(lines, name)
				} else {
					size := fmt.Sprintf("%d", info.Size())
					lines = append(lines, fmt.Sprintf("%-20s %10s", name, size))
				}
			} else {
				lines = append(lines, name)
			}
		}
		return strings.Join(lines, "\n")

	// Очистка терминала (сброс history и вывод "$ ")
	case "clear":
		return "\f" // спецсимвол: в renderTerminal он очистит окно

	case "pwd":
		wd, _ := os.Getwd()
		return wd

	case "cd":
		if len(parts) < 2 {
			return "Использование: cd <dir>"
		}
		if err := os.Chdir(parts[1]); err != nil {
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
		if err := os.Mkdir(parts[1], 0755); err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "touch":
		if len(parts) < 2 {
			return "Использование: touch <file>"
		}
		f, err := os.Create(parts[1])
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		_ = f.Close()
		return ""

	case "rm":
		if len(parts) < 2 {
			return "Использование: rm <file>"
		}
		if err := os.Remove(parts[1]); err != nil {
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
		if _, err := io.Copy(out, in); err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "mv":
		if len(parts) < 3 {
			return "Использование: mv <src> <dst>"
		}
		if err := os.Rename(parts[1], parts[2]); err != nil {
			return "Ошибка: " + err.Error()
		}
		return ""

	case "echo":
		return strings.Join(parts[1:], " ")

	// Пинг с вызовом системной утилиты
	case "ping":
		if len(parts) < 2 {
			return "Использование: ping <host>"
		}
		c := exec.Command("ping", "-c", "4", parts[1])
		out, err := c.CombinedOutput()
		if err != nil {
			return "Ошибка: " + err.Error() + "\n" + string(out)
		}
		return string(out)

	// Справка
	case "help":
		return `Доступные команды:
  ls [-a] [-l] [dir]   - список файлов
  clear                - очистить экран
  pwd                  - показать текущую директорию
  cd <dir>             - сменить директорию
  cat <file>           - показать содержимое файла
  mkdir <dir>          - создать директорию
  touch <file>         - создать пустой файл
  rm <file>            - удалить файл
  cp <src> <dst>       - копировать файл
  mv <src> <dst>       - переместить/переименовать файл
  echo <text>          - вывести текст
  ping <host>          - проверить доступность адреса
  help                 - показать эту справку`

	default:
		// Внешняя команда
		c := exec.Command(parts[0], parts[1:]...)
		out, err := c.CombinedOutput()
		if err != nil {
			return "Ошибка: " + err.Error() + "\n" + string(out)
		}
		return string(out)
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

func truncateToWidth(s string, max int) string {
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > max {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String()
}

func truncateFromRight(s string, max int) string {
	rs := []rune(s)
	widths := make([]int, len(rs))
	total := 0
	for i, r := range rs {
		w := runewidth.RuneWidth(r)
		widths[i] = w
		total += w
	}
	if total <= max {
		return s
	}
	index := 0
	for total > max && index < len(rs) {
		total -= widths[index]
		index++
	}
	return string(rs[index:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
