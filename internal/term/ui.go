package term

// Simple ANSI-colored prefixes for user and assistant messages.

const (
	colorCyan  = "\033[36m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

func UserPrefix() string {
	return colorCyan + "you> " + colorReset
}

func AssistantPrefix() string {
	return colorGreen + "koba> " + colorReset
}

