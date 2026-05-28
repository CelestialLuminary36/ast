package main

import "testing"

func TestParseScenariosFlag(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--scenarios=./foo"}, "./foo"},
		{[]string{"--scenarios=./bar"}, "./bar"},
		{[]string{"--scenarios="}, ""},
	}
	for _, c := range cases {
		got := parseScenariosFlag(c.args)
		if got != c.want {
			t.Errorf("parseScenariosFlag(%v) = %q; want %q", c.args, got, c.want)
		}
	}
}

func TestWantsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"empty args", nil, false},
		{"--help present", []string{"./skills/foo", "--help"}, true},
		{"-h present", []string{"-h"}, true},
		{"bare 'help' word", []string{"help"}, true},
		{"help appears after positional", []string{"./skills/foo", "help"}, true},
		{"unrelated flag", []string{"--scenarios=./s"}, false},
		// We do NOT want substrings like 'helpful' to trigger help.
		{"substring match must not trigger", []string{"helpful"}, false},
		{"--helper must not trigger", []string{"--helper"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wantsHelp(c.args); got != c.want {
				t.Errorf("wantsHelp(%v) = %v; want %v", c.args, got, c.want)
			}
		})
	}
}

