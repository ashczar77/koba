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

// StartSpinner starts an animated spinner on w with the given message. It returns
// a stop function that clears the line and stops the spinner. Call it when the
// response starts or on error.
func StartSpinner(w io.Writer, message string) (stop func()) {
	var done bool
	var mu sync.Mutex
	tick := time.NewTicker(80 * time.Millisecond)
	go func() {
		i := 0
		for {
			select {
			case <-tick.C:
				mu.Lock()
				if done {
					mu.Unlock()
					return
				}
				frame := spinnerFrames[i%len(spinnerFrames)]
				i++
				mu.Unlock()
				fmt.Fprintf(w, "\r%s%s %s%s", colorGreen, frame, colorReset, message)
				if f, ok := w.(interface{ Flush() error }); ok {
					_ = f.Flush()
				}
			}
		}
	}()
	return func() {
		mu.Lock()
		done = true
		mu.Unlock()
		tick.Stop()
		fmt.Fprint(w, "\r\033[K")
		if f, ok := w.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
	}
}

