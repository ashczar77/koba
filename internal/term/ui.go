package term

import (
	"fmt"
	"io"
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

func ColorCyan() string    { return colorCyan }
func ColorGreen() string  { return colorGreen }
func ColorMagenta() string { return colorMagenta }
func ColorDim() string    { return colorDim }
func ColorReset() string  { return colorReset }
func ColorRed() string    { return colorRed }
func ColorYellow() string { return colorYellow }

const bannerInnerWidth = 72

var ansiStrip = regexp.MustCompile(`\033\[[0-9;]*m`)

func UserPrefix() string {
	return colorCyan + "you " + colorReset + "▸ "
}

func AssistantPrefix() string {
	return colorGreen + "koba" + colorReset + " ▸ "
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

	// Big ASCII art for KOBA
	logo := []string{
		fmt.Sprintf("%s ██╗  ██╗ ██████╗ ██████╗  █████╗ %s", colorMagenta, colorReset),
		fmt.Sprintf("%s ██║ ██╔╝██╔═══██╗██╔══██╗██╔══██╗%s", colorMagenta, colorReset),
		fmt.Sprintf("%s █████╔╝ ██║   ██║██████╔╝███████║%s", colorMagenta, colorReset),
		fmt.Sprintf("%s ██╔═██╗ ██║   ██║██╔══██╗██╔══██║%s", colorMagenta, colorReset),
		fmt.Sprintf("%s ██║  ██╗╚██████╔╝██████╔╝██║  ██║%s", colorMagenta, colorReset),
		fmt.Sprintf("%s ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═╝%s", colorMagenta, colorReset),
	}

	tagline := fmt.Sprintf("%sFuturistic coding companion in your terminal%s", colorDim, colorReset)

	// Truncate model if too long for status line
	if len(model) > 28 {
		model = model[:25] + "..."
	}
	status := fmt.Sprintf("%s●%s Mode: %s%-5s%s  Provider: %s%-8s%s  Model: %s%s%s",
		colorGreen, colorReset,
		colorCyan, mode, colorReset,
		colorMagenta, provider, colorReset,
		colorGreen, model, colorReset,
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

	help := fmt.Sprintf("%sType your message and press Enter. Ctrl+D to exit.%s", colorDim, colorReset)
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
			out = append(out, colorDim+line+colorReset)
		} else if strings.HasPrefix(line, "@@") {
			out = append(out, colorYellow+line+colorReset)
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			out = append(out, colorRed+line+colorReset)
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			out = append(out, colorGreen+line+colorReset)
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// FormatDiffBlock renders a proposed diff with a styled header and optional footer.
func FormatDiffBlock(diff string, dryRun bool) string {
	sep := colorDim + "────────────────────────────────────────────────────────────────────────" + colorReset
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(colorMagenta + " Proposed diff " + colorReset + "\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(FormatDiff(diff) + "\n")
	sb.WriteString(sep + "\n")
	if dryRun {
		sb.WriteString(colorYellow + " (dry-run: diff not applied) " + colorReset + "\n")
	}
	return sb.String()
}

// FormatReview formats review output with section headers and spacing.
func FormatReview(text string) string {
	// Style numbered section starts (1. 2. 3. 4.) at line start
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "1. ") || strings.HasPrefix(trimmed, "2. ") ||
			strings.HasPrefix(trimmed, "3. ") || strings.HasPrefix(trimmed, "4. ") {
			out = append(out, "")
			out = append(out, colorMagenta+trimmed+colorReset)
		} else if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") {
			out = append(out, "  "+colorDim+trimmed+colorReset)
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
		sb.WriteString(colorDim + "┌─ code ─────────────────────────────────────────────────────────────┐" + colorReset + "\n")
		for _, l := range block {
			sb.WriteString(colorGreen + l + colorReset + "\n")
		}
		sb.WriteString(colorDim + "└──────────────────────────────────────────────────────────────────┘" + colorReset + "\n")
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
				fmt.Fprintf(w, "\r%s%s %s%s", colorGreen, frame, colorReset, message)
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

