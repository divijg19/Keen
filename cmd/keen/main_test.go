package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/divijg19/Keen/internal/keen"
)

func TestParseArgsModeSelection(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
		rich    bool
		inter   bool
	}{
		{name: "default is canonical", args: nil, rich: false, inter: false},
		{name: "rich via -r", args: []string{"-r"}, rich: true, inter: false},
		{name: "interactive via -i", args: []string{"-i"}, rich: false, inter: true},
		{name: "rich long form", args: []string{"--r=false", "-i"}, rich: false, inter: true},
		{name: "overlap rejected", args: []string{"-r", "-i"}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts, err := parseArgs(c.args)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (opts=%+v)", opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Rich != c.rich {
				t.Errorf("Rich = %v, want %v", opts.Rich, c.rich)
			}
			if opts.Interactive != c.inter {
				t.Errorf("Interactive = %v, want %v", opts.Interactive, c.inter)
			}
		})
	}
}

func TestParseArgsSelectionInvariance(t *testing.T) {
	// Selection flags must behave identically regardless of presentation mode.
	for _, mode := range [][]string{nil, {"-r"}, {"-i"}} {
		opts, err := parseArgs(append([]string{"--clean", "--recent", "7d"}, mode...))
		if err != nil {
			t.Fatalf("mode %v: unexpected error: %v", mode, err)
		}
		if !opts.ShowClean || !opts.HasRecent {
			t.Errorf("mode %v: selection flags not applied: %+v", mode, opts)
		}
		if opts.Rich != (len(mode) == 1 && mode[0] == "-r") {
			t.Errorf("mode %v: unexpected Rich value", mode)
		}
		if opts.Interactive != (len(mode) == 1 && mode[0] == "-i") {
			t.Errorf("mode %v: unexpected Interactive value", mode)
		}
	}
}

func TestParseArgsRecentValidation(t *testing.T) {
	if _, err := parseArgs([]string{"--recent", "7d"}); err != nil {
		t.Errorf("valid --recent should not error: %v", err)
	}
	if _, err := parseArgs([]string{"--recent", "not-a-duration"}); err == nil {
		t.Errorf("invalid --recent should error")
	}
}

func TestParseArgsInvalidFlag(t *testing.T) {
	if _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Errorf("unknown flag should error")
	}
}

// TestParseArgsRejectsCompactWithModes pins the v0.5.7 grammar rule: --compact
// is a modifier of the canonical report and must be rejected alongside any
// presentation mode, before discovery or enrichment runs.
func TestParseArgsRejectsCompactWithModes(t *testing.T) {
	for _, args := range [][]string{
		{"-r", "--compact"},
		{"--compact", "-r"},
		{"-i", "--compact"},
		{"--compact", "-i"},
	} {
		if opts, err := parseArgs(args); err == nil {
			t.Errorf("args %v should error, got opts=%+v", args, opts)
		}
	}
}

// TestParseArgsRejectsCombinedShortFlags pins the existing Go flag-package
// behavior that v0.5.7 must not alter: combined short flags are not a
// supported syntax and fail at parse time without producing a report.
func TestParseArgsRejectsCombinedShortFlags(t *testing.T) {
	for _, args := range [][]string{{"-ri"}, {"-ir"}} {
		opts, err := parseArgs(args)
		if err == nil {
			t.Errorf("combined short flags %v must not parse, got opts=%+v", args, opts)
		}
	}
}

func TestParseArgsCompactRemainsCanonicalOnly(t *testing.T) {
	// Canonical combinations stay valid.
	for _, args := range [][]string{
		{"--compact"},
		{"--clean", "--compact"},
		{"--dirty", "--compact"},
		{"--recent", "7d", "--compact"},
	} {
		if _, err := parseArgs(args); err != nil {
			t.Errorf("args %v must remain valid: %v", args, err)
		}
	}
	// Selection flags remain valid with modes.
	for _, mode := range []string{"-r", "-i"} {
		if _, err := parseArgs([]string{mode, "--dirty"}); err != nil {
			t.Errorf("%s --dirty must remain valid: %v", mode, err)
		}
	}
}

// TestInteractiveNonTTYNoEscape verifies the portability contract: with stdin
// redirected to a pipe (not a terminal), `keen -i` emits exactly one one-shot
// List render with no terminal escape/control sequences and exits normally.
// The subprocess re-executes the test binary with a helper flag so it observes
// a real non-terminal stdin via os.Stdin.
func TestInteractiveNonTTYNoEscape(t *testing.T) {
	if os.Getenv("KEEN_NONTTY_HELPER") == "1" {
		keen.Browse([]keen.Repository{{Name: "peony"}}, 1)
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=TestInteractiveNonTTYNoEscape")
	cmd.Env = append(os.Environ(), "KEEN_NONTTY_HELPER=1")
	cmd.Stdin = strings.NewReader("")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess failed: %v\nstderr: %s", err, errb.String())
	}

	if out.Len() == 0 {
		t.Fatal("expected one-shot List output, got none")
	}
	for _, r := range out.String() {
		if r == '\x1b' || r == '\x03' || (r < 0x20 && r != '\n' && r != '\t' && r != '\r') {
			t.Fatalf("terminal escape/control sequence leaked into non-TTY output: %q", r)
		}
	}
	if !strings.Contains(out.String(), "‹ LIST ›") {
		t.Errorf("one-shot output missing List header: %q", out.String())
	}
}
