package main

import "testing"

func TestParseScenariosFlag(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--scenarios=./foo"}, "./foo"},
		{[]string{"--runner=api", "--scenarios=./bar"}, "./bar"},
		{[]string{"--scenarios="}, ""},
	}
	for _, c := range cases {
		got := parseScenariosFlag(c.args)
		if got != c.want {
			t.Errorf("parseScenariosFlag(%v) = %q; want %q", c.args, got, c.want)
		}
	}
}

func TestParseRunnerFlag(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--runner=api"}, "api"},
		{[]string{"--scenarios=./foo", "--runner=mock"}, "mock"},
		{[]string{"--runner="}, ""},
	}
	for _, c := range cases {
		got := parseRunnerFlag(c.args)
		if got != c.want {
			t.Errorf("parseRunnerFlag(%v) = %q; want %q", c.args, got, c.want)
		}
	}
}
