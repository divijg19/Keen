//go:build linux

package keen

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// Manual termios layout for linux/amd64. The standard library's syscall
// package does not expose Tcgetattr/Tcsetattr or Winsize on Linux, and pulling
// in golang.org/x/sys would add a dependency. The offsets below were verified
// against the host headers (sizeof=60, c_cc at 17, NCCS=32, VMIN=6, VTIME=5).
//
//	struct termios {
//	  tcflag_t c_iflag;  // 0
//	  tcflag_t c_oflag;  // 4
//	  tcflag_t c_cflag;  // 8
//	  tcflag_t c_lflag;  // 12
//	  cc_t     c_line;   // 16
//	  cc_t     c_cc[32]; // 17
//	} (padded to 60)
type termios struct {
	Iflag uint32
	Oflag uint32
	Cflag uint32
	Lflag uint32
	Line  uint8
	Cc    [32]uint8
	Pad   [11]uint8
}

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

const (
	tcgets     = 0x5401
	tcsets     = 0x5402
	tiocgwinsz = 0x5413

	istrip = 0o40
	inlcr  = 0o100
	icrnl  = 0o400
	igncr  = 0o200
	ixon   = 0o2000
	ixoff  = 0o10000

	echo   = 0o10
	icanon = 0o2
	isig   = 0o1

	ccVMIN  = 6
	ccVTIME = 5
)

var savedTerm termios

// makeRaw switches the terminal into canonical-off, echo-off mode so individual
// key presses (including escape sequences) can be read without waiting for
// Enter. It returns an error when fd is not a terminal, which lets Browse fall
// back to a static render.
func makeRaw(fd int) error {
	var t termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tcgets, uintptr(unsafe.Pointer(&t)))
	if errno != 0 {
		return errno
	}
	savedTerm = t

	t.Iflag &^= istrip | inlcr | icrnl | igncr | ixon | ixoff
	t.Lflag &^= echo | icanon | isig
	t.Cc[ccVMIN] = 1
	t.Cc[ccVTIME] = 0

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tcsets, uintptr(unsafe.Pointer(&t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func restoreRaw(fd int) {
	if savedTerm == (termios{}) {
		return
	}
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tcsets, uintptr(unsafe.Pointer(&savedTerm)))
}

// readKey blocks for one input event. The raw sequence accompanies the
// normalized action so callers (notably filter editing) can consume printable
// characters. Multi-byte UTF-8 input arrives as one event: after a sequence
// leader the remaining continuation bytes are read (blocking, as the terminal
// delivers a keystroke atomically) so CJK, emoji, and accented query text is
// never fragmented into invalid partials. The boolean reports whether the
// input stream is still alive; when it is false (EOF, closed or hung-up tty,
// I/O error) the caller must terminate the interactive session so terminal
// restoration runs instead of spinning on a dead stream.
func readKey() (keyAction, string, bool) {
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return keyNone, "", false
	}
	if buf[0] == 0x1b {
		raw := "\x1b" + readTrailing()
		return interpretSequence(raw), raw, true
	}
	raw := []byte{buf[0]}
	// Hoist the sequence length: buf is reused for continuation reads, so
	// re-evaluating the leader mid-loop would see continuation bytes.
	need := utf8SequenceLen(buf[0])
	for len(raw) < need {
		m, rerr := os.Stdin.Read(buf)
		if rerr != nil || m == 0 {
			break
		}
		raw = append(raw, buf[0])
	}
	s := string(raw)
	return interpretSequence(s), s, true
}

// readTrailing consumes up to eight more bytes after an ESC without blocking,
// so arrow-key sequences (including modified forms like ESC [ 1 ; 2 D) are
// captured as a single event.
func readTrailing() string {
	fd := int(os.Stdin.Fd())
	_ = syscall.SetNonblock(fd, true)
	defer func() { _ = syscall.SetNonblock(fd, false) }()

	var sb strings.Builder
	buf := make([]byte, 1)
	for i := 0; i < 8; i++ {
		n, err := os.Stdin.Read(buf)
		if n == 1 {
			sb.WriteByte(buf[0])
			continue
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}

func terminalWidth() int {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), tiocgwinsz, uintptr(unsafe.Pointer(&ws)))
	if errno == 0 && ws.Col > 0 {
		return int(ws.Col)
	}
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, convErr := strconv.Atoi(c); convErr == nil && n > 0 {
			return n
		}
	}
	return 80
}

func terminalHeight() int {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), tiocgwinsz, uintptr(unsafe.Pointer(&ws)))
	if errno == 0 && ws.Row > 0 {
		return int(ws.Row)
	}
	if c := os.Getenv("LINES"); c != "" {
		if n, convErr := strconv.Atoi(c); convErr == nil && n > 0 {
			return n
		}
	}
	return 24
}
