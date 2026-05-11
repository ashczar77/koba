package term

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Simple ANSI-colored prefixes and layout helpers for the terminal UI.

const (
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorMagenta = "\033[35m"
	colorDim     = "\033[2m"
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
)

// colorEnabled reports whether color output should be used.
// Respects NO_COLOR env var and checks if stdout is a terminal.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
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

const bannerInnerWidth = 72

var ansiStrip = regexp.MustCompile(`\033\[[0-9;]*m`)

func UserPrefix() string {
	return color(colorCyan) + "you " + color(colorReset) + "▸ "
}

func AssistantPrefix() string {
	return color(colorGreen) + "koba" + color(colorReset) + " ▸ "
}

// visibleLen returns the number of visible runes (ANSI codes stripped).
func visibleLen(s string) int {
	return utf8.RuneCountInString(ansiStrip.ReplaceAllString(s, ""))
}

// centerLine centers s within the banner width. Padding uses visible length.
func centerLine(s string) string {
	vis := visibleLen(s)
	if vis > bannerInnerWidth {
		// Truncate: keep prefix, lose end (including any trailing ANSI)
		runes := []rune(ansiStrip.ReplaceAllString(s, ""))
		s = string(runes[:bannerInnerWidth-3]) + "..."
		vis = bannerInnerWidth
	}
	pad := bannerInnerWidth - vis
	left := pad / 2
	right := pad - left
	return "│ " + strings.Repeat(" ", left) + s + strings.Repeat(" ", right) + " │"
}

// padLine left-pads content and ensures total visible width fits; truncates model if needed.
func padLine(s string) string {
	vis := visibleLen(s)
	if vis > bannerInnerWidth {
		runes := []rune(ansiStrip.ReplaceAllString(s, ""))
		s = string(runes[:bannerInnerWidth-3]) + "..."
	}
	return "│ " + s + strings.Repeat(" ", bannerInnerWidth-visibleLen(s)) + " │"
}

// Banner renders a portal-style header for the session.
func Banner(provider, model, mode string) string {
	top := "┌" + strings.Repeat("─", bannerInnerWidth+2) + "┐"
	bot := "└" + strings.Repeat("─", bannerInnerWidth+2) + "┘"

	logo := []string{
		fmt.Sprintf("%s ██╗  ██╗ ██████╗ ██████╗  █████╗ %s", color(colorMagenta), color(colorReset)),
		fmt.Sprintf("%s ██║ ██╔╝██╔═══██╗██╔══██╗██╔══██╗%s", color(colorMagenta), color(colorReset)),
		fmt.Sprintf("%s █████╔╝ ██║   ██║██████╔╝███████║%s", color(colorMagenta), color(colorReset)),
		fmt.Sprintf("%s ██╔═██╗ ██║   ██║██╔══██╗██╔══██║%s", color(colorMagenta), color(colorReset)),
		fmt.Sprintf("%s ██║  ██╗╚██████╔╝██████╔╝██║  ██║%s", color(colorMagenta), color(colorReset)),
		fmt.Sprintf("%s ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═╝%s", color(colorMagenta), color(colorReset)),
	}

	tagline := fmt.Sprintf("%sFuturistic coding companion in your terminal%s", color(colorDim), color(colorReset))

	if len(model) > 28 {
		model = model[:25] + "..."
	}
	status := fmt.Sprintf("%s●%s Mode: %s%-5s%s  Provider: %s%-8s%s  Model: %s%s%s",
		color(colorGreen), color(colorReset),
		color(colorCyan), mode, color(colorReset),
		color(colorMagenta), provider, color(colorReset),
		color(colorGreen), model, color(colorReset),
	)

	lines := []string{top}
	for _, l := range logo {
		lines = append(lines, centerLine(l))
	}
	lines = append(lines, centerLine(""))
	lines = append(lines, centerLine(tagline))
	lines = append(lines, centerLine(""))
	lines = append(lines, padLine(status))
	lines = append(lines, bot)

	help := fmt.Sprintf("%sType your message and press Enter. Ctrl+D to exit.%s", color(colorDim), color(colorReset))
	return strings.Join(lines, "\n") + "\n\n" + help + "\n"
}

// Spinner frames for a "thinking" animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// FormatDiff colorizes a unified diff string for terminal output.
func FormatDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			out = append(out, color(colorDim)+line+color(colorReset))
		} else if strings.HasPrefix(line, "@@") {
			out = append(out, color(colorYellow)+line+color(colorReset))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			out = append(out, color(colorRed)+line+color(colorReset))
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			out = append(out, color(colorGreen)+line+color(colorReset))
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// FormatDiffBlock renders a proposed diff with a styled header and optional footer.
func FormatDiffBlock(diff string, dryRun bool) string {
	sep := color(colorDim) + "────────────────────────────────────────────────────────────────────────" + color(colorReset)
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(color(colorMagenta) + " Proposed diff " + color(colorReset) + "\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(FormatDiff(diff) + "\n")
	sb.WriteString(sep + "\n")
	if dryRun {
		sb.WriteString(color(colorYellow) + " (dry-run: diff not applied) " + color(colorReset) + "\n")
	}
	return sb.String()
}

// FormatReview formats review output with section headers and spacing.
func FormatReview(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "1. ") || strings.HasPrefix(trimmed, "2. ") ||
			strings.HasPrefix(trimmed, "3. ") || strings.HasPrefix(trimmed, "4. ") {
			out = append(out, "")
			out = append(out, color(colorMagenta)+trimmed+color(colorReset))
		} else if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") {
			out = append(out, "  "+color(colorDim)+trimmed+color(colorReset))
		} else {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}

// FormatResponse formats ask/code output: wrap code blocks in a subtle box.
func FormatResponse(text string) string {
	const codeFence = "```"
	var sb strings.Builder
	lines := strings.Split(text, "\n")
	inBlock := false
	var block []string

	flushBlock := func() {
		if len(block) == 0 {
			return
		}
		sb.WriteString(color(colorDim) + "┌─ code ─────────────────────────────────────────────────────────────┐" + color(colorReset) + "\n")
		for _, l := range block {
			sb.WriteString(color(colorGreen) + l + color(colorReset) + "\n")
		}
		sb.WriteString(color(colorDim) + "└──────────────────────────────────────────────────────────────────┘" + color(colorReset) + "\n")
		block = block[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == codeFence || strings.HasPrefix(trimmed, codeFence) {
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
		sb.WriteString(line + "\n")
	}
	flushBlock()
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// StartSpinner starts an animated spinner on w with the given message. It returns
// a stop function that clears the line and stops the spinner. Call it when the
// response starts or on error. Stop blocks until the spinner goroutine has
// fully exited, so no \r overwrites can occur after stop returns.
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
				fmt.Fprintf(w, "\r%s%s %s%s", color(colorGreen), frame, color(colorReset), message)
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
			mu.Unlock() // wait for any in-flight write to finish
			fmt.Fprint(w, "\r\033[K")
			if f, ok := w.(interface{ Flush() error }); ok {
				_ = f.Flush()
			}
		})
	}
}

