package term

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ANSI color codes.
const (
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorMagenta = "\033[35m"
	colorDim     = "\033[2m"
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorBold    = "\033[1m"
	colorBlue    = "\033[34m"
)

var colorEnabledOnce sync.Once
var colorEnabledVal bool

func colorEnabled() bool {
	colorEnabledOnce.Do(func() {
		if os.Getenv("NO_COLOR") != "" {
			colorEnabledVal = false
			return
		}
		fi, err := os.Stdout.Stat()
		if err != nil {
			colorEnabledVal = false
			return
		}
		colorEnabledVal = (fi.Mode() & os.ModeCharDevice) != 0
	})
	return colorEnabledVal
}

func color(c string) string {
	if !colorEnabled() {
		return ""
	}
	return c
}

func ColorCyan() string    { return color(colorCyan) }
func ColorGreen() string   { return color(colorGreen) }
func ColorMagenta() string { return color(colorMagenta) }
func ColorDim() string     { return color(colorDim) }
func ColorReset() string   { return color(colorReset) }
func ColorRed() string     { return color(colorRed) }
func ColorYellow() string  { return color(colorYellow) }

var ansiStrip = regexp.MustCompile(`\033\[[0-9;]*m`)

// ShortenPath returns a compact cwd: ~/foo or last 2 segments if deep.
func ShortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		path = filepath.Join(parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}

// UserPrefix returns the colored user prompt with shortened cwd.
func UserPrefix() string {
	cwd, _ := os.Getwd()
	short := ShortenPath(cwd)
	return fmt.Sprintf("%s%s%s %s❯%s ", color(colorDim), short, color(colorReset), color(colorCyan), color(colorReset))
}

// AssistantPrefix returns the colored assistant prompt.
func AssistantPrefix() string {
	return color(colorMagenta) + "koba" + color(colorReset) + " › "
}

// ToolPrefix returns a subtle tool indicator.
func ToolPrefix(toolName, detail string) string {
	return fmt.Sprintf("  %s⚙ %s%s %s%s\n", color(colorDim), toolName, color(colorReset), color(colorDim)+detail+color(colorReset), "")
}

// Banner renders a compact, clean header for the session.
func Banner(provider, model, mode string) string {
	if len(model) > 30 {
		model = model[:27] + "..."
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s%skoba%s", color(colorBold), color(colorMagenta), color(colorReset)))
	sb.WriteString(fmt.Sprintf(" %s— your coding companion%s\n", color(colorDim), color(colorReset)))
	sb.WriteString(fmt.Sprintf("  %s%s • %s • %s%s\n", color(colorDim), mode, provider, model, color(colorReset)))
	sb.WriteString(fmt.Sprintf("  %sType a message to begin. Ctrl+D to exit.%s\n\n", color(colorDim), color(colorReset)))
	return sb.String()
}

// ExitMessage returns a clean goodbye.
func ExitMessage() string {
	return fmt.Sprintf("\n  %s👋 See you next time.%s\n", color(colorDim), color(colorReset))
}

// FormatDiff colorizes a unified diff string for terminal output.
func FormatDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var out []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			out = append(out, color(colorDim)+line+color(colorReset))
		case strings.HasPrefix(line, "@@"):
			out = append(out, color(colorYellow)+line+color(colorReset))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			out = append(out, color(colorRed)+line+color(colorReset))
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out = append(out, color(colorGreen)+line+color(colorReset))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// FormatDiffBlock renders a proposed diff with a styled header.
func FormatDiffBlock(diff string, dryRun bool) string {
	sep := color(colorDim) + strings.Repeat("─", 60) + color(colorReset)
	var sb strings.Builder
	sb.WriteString("\n" + color(colorMagenta) + " Proposed diff" + color(colorReset) + "\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(FormatDiff(diff) + "\n")
	sb.WriteString(sep + "\n")
	if dryRun {
		sb.WriteString(color(colorYellow) + " (dry-run: not applied)" + color(colorReset) + "\n")
	}
	return sb.String()
}

// FormatReview formats review output with section headers.
func FormatReview(text string) string {
	return FormatResponse(text)
}

// FormatResponse renders markdown-like formatting: headers, bold, code blocks, lists.
func FormatResponse(text string) string {
	const codeFence = "```"
	width := termWidth()
	var sb strings.Builder
	lines := strings.Split(text, "\n")
	inBlock := false
	var block []string

	flushBlock := func() {
		if len(block) == 0 {
			return
		}
		sb.WriteString(color(colorDim) + "  ┌──────────────────────────────────────────────────────────┐" + color(colorReset) + "\n")
		for _, l := range block {
			sb.WriteString(color(colorGreen) + "  │ " + l + color(colorReset) + "\n")
		}
		sb.WriteString(color(colorDim) + "  └──────────────────────────────────────────────────────────┘" + color(colorReset) + "\n")
		block = block[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, codeFence) {
			if inBlock {
				flushBlock()
				inBlock = false
			} else {
				inBlock = true
			}
			continue
		}
		if inBlock {
			block = append(block, line)
			continue
		}
		// Markdown headers
		if strings.HasPrefix(trimmed, "### ") {
			sb.WriteString(color(colorBold) + "  " + trimmed[4:] + color(colorReset) + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			sb.WriteString(color(colorBold) + color(colorCyan) + trimmed[3:] + color(colorReset) + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			sb.WriteString(color(colorBold) + color(colorMagenta) + trimmed[2:] + color(colorReset) + "\n")
			continue
		}
		// Lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			content := "  • " + trimmed[2:]
			sb.WriteString(wordWrap(content, width) + "\n")
			continue
		}
		// Numbered lists (keep as-is but indent)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:3], ".") {
			content := "  " + trimmed
			sb.WriteString(wordWrap(content, width) + "\n")
			continue
		}
		// Regular text with inline formatting
		sb.WriteString(wordWrap(line, width) + "\n")
	}
	flushBlock()
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

var boldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
var inlineCodeRe = regexp.MustCompile("`([^`]+)`")

func renderInlineBold(s string) string {
	return boldRe.ReplaceAllString(s, color(colorBold)+"$1"+color(colorReset))
}

func renderInlineCode(s string) string {
	return inlineCodeRe.ReplaceAllString(s, color(colorCyan)+"$1"+color(colorReset))
}

// termWidth returns the terminal width, defaulting to 80.
func termWidth() int {
	if w := os.Getenv("COLUMNS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			return n
		}
	}
	// Try ioctl
	if f, err := os.Open("/dev/tty"); err == nil {
		defer f.Close()
		if ws, err := getWinSize(f); err == nil && ws > 0 {
			return ws
		}
	}
	return 80
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getWinSize(f *os.File) (int, error) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0, errno
	}
	return int(ws.Col), nil
}

// wordWrap wraps text to fit within maxWidth, keeping words whole.
// It preserves leading indent.
func wordWrap(s string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}
	// Strip ANSI to measure visible length, but we wrap the original.
	visible := ansiStrip.ReplaceAllString(s, "")
	if len(visible) <= maxWidth {
		return s
	}

	// Determine leading indent from original visible text.
	indent := ""
	for _, r := range visible {
		if r == ' ' || r == '\t' {
			indent += string(r)
		} else {
			break
		}
	}
	// For continuation lines, add 2 extra spaces.
	contIndent := indent + "  "
	if len(contIndent) >= maxWidth/2 {
		contIndent = indent
	}

	words := strings.Fields(visible)
	if len(words) == 0 {
		return s
	}

	var lines []string
	line := indent + words[0]
	lineLen := len(indent) + len(words[0])

	for _, w := range words[1:] {
		if lineLen+1+len(w) > maxWidth {
			lines = append(lines, line)
			line = contIndent + w
			lineLen = len(contIndent) + len(w)
		} else {
			line += " " + w
			lineLen += 1 + len(w)
		}
	}
	lines = append(lines, line)

	// Re-apply inline formatting to each wrapped line.
	var result []string
	for _, l := range lines {
		l = renderInlineBold(l)
		l = renderInlineCode(l)
		result = append(result, l)
	}
	return strings.Join(result, "\n")
}

// Spinner frames.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner starts an animated spinner. Returns a stop function.
func StartSpinner(w io.Writer, message string) (stop func()) {
	stopCh := make(chan struct{})
	exited := make(chan struct{})
	var mu sync.Mutex
	tick := time.NewTicker(80 * time.Millisecond)
	go func() {
		defer close(exited)
		i := 0
		for {
			select {
			case <-stopCh:
				return
			case <-tick.C:
				select {
				case <-stopCh:
					return
				default:
				}
				mu.Lock()
				select {
				case <-stopCh:
					mu.Unlock()
					return
				default:
				}
				frame := spinnerFrames[i%len(spinnerFrames)]
				i++
				fmt.Fprintf(w, "\r  %s%s %s%s", color(colorMagenta), frame, message, color(colorReset))
				if f, ok := w.(interface{ Flush() error }); ok {
					_ = f.Flush()
				}
				mu.Unlock()
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stopCh)
			tick.Stop()
			<-exited
			mu.Lock()
			mu.Unlock()
			fmt.Fprint(w, "\r\033[K")
			if f, ok := w.(interface{ Flush() error }); ok {
				_ = f.Flush()
			}
		})
	}
}
