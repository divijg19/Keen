//go:build !linux

package keen

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// makeRaw is a no-op on platforms without the unix terminal API. Browse
// detects the failure and renders a single static page instead of looping.
func makeRaw(fd int) error {
	return nil
}

func restoreRaw(fd int) {}

// readKey falls back to line-based input: one line equals one navigation
// event. This keeps the browser usable (if awkward) on unsupported platforms
// without pulling in a third-party terminal library. The boolean reports
// whether the input stream is still alive; a false value ends the session.
func readKey() (keyAction, bool) {
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil && s == "" {
		// Stream ended or failed permanently (EOF on closed stdin, I/O
		// error). Signal termination so the caller cannot busy-loop.
		return keyNone, false
	}
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return keyNone, true
	}
	return interpretSequence(s), true
}

func terminalWidth() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
