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

