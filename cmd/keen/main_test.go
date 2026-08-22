package main

import (
	"testing"
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
