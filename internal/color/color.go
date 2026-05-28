// Package color provides terminal color utilities for ast output.
// Respects NO_COLOR, FORCE_COLOR, and TERM=dumb conventions.
package color

import "os"

// Enabled controls whether ANSI escape codes are emitted. Defaults to
// auto-detect: enabled when TERM is set to something other than "dumb",
// unless NO_COLOR is set. FORCE_COLOR overrides. Callers can set this
// to false to implement --no-color.
var Enabled bool

func init() {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	if os.Getenv("FORCE_COLOR") != "" {
		Enabled = true
		return
	}
	term := os.Getenv("TERM")
	if term != "" && term != "dumb" {
		Enabled = true
	}
}

// Reset returns the ANSI reset sequence, or empty if color is disabled.
func Reset() string {
	if !Enabled {
		return ""
	}
	return "\033[0m"
}

func Green(s string) string  { return color("32", s) }
func Red(s string) string    { return color("31", s) }
func Yellow(s string) string { return color("33", s) }
func Cyan(s string) string   { return color("36", s) }
func Bold(s string) string   { return color("1", s) }

func color(code, s string) string {
	if !Enabled {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}
