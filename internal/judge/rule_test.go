package judge

import (
	"strings"
	"testing"

	"github.com/CelestialLuminary36/ast/internal/runner"
	"github.com/CelestialLuminary36/ast/internal/scenario"
)

func TestMatchGlob_Doublestar(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// 基础 *
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},

		// 前缀型 **（旧实现也能处理）
		{"src/**", "src/a.go", true},
		{"src/**", "src/a/b.go", true},
		{"src/**", "other/a.go", false},

		// 跨目录 **/*.ext —— 旧实现漏判的核心场景
		{"src/**/*.go", "src/a.go", true},
		{"src/**/*.go", "src/a/b.go", true},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "src/a.txt", false},
		{"src/**/*.go", "other/a.go", false},

		// 双 **
		{"**/test/**", "a/test/b.go", true},
		{"**/test/**", "test/a.go", true},
		{"**/test/**", "a/b/c.go", false},

		// 精确路径
		{"package-lock.json", "package-lock.json", true},
		{"package-lock.json", "sub/package-lock.json", false},
	}

	for _, c := range cases {
		got := matchGlob(c.pattern, c.path)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v; want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestAuditFileContent(t *testing.T) {
	j := NewRuleJudge()

	mkResult := func(files map[string]string) *runner.RunResult {
		var names []string
		for k := range files {
			names = append(names, k)
		}
		return &runner.RunResult{MutatedFiles: names, FileContents: files}
	}

	cases := []struct {
		name        string
		files       map[string]string
		rules       []scenario.FileContentAssert
		wantPassed  bool
		wantErrSubs []string // substrings expected in the error messages
	}{
		{
			name:  "must_match present in at-least-one mode",
			files: map[string]string{"foo_test.go": `errors.Is(err, ErrEmptyInput)`},
			rules: []scenario.FileContentAssert{{
				Glob:      "*_test.go",
				MustMatch: []string{`errors\.Is\(.*ErrEmptyInput\)`},
			}},
			wantPassed: true,
		},
		{
			name:  "must_match missing fails",
			files: map[string]string{"foo_test.go": `if err == ErrEmptyInput {}`},
			rules: []scenario.FileContentAssert{{
				Glob:      "*_test.go",
				MustMatch: []string{`errors\.Is\(`},
			}},
			wantPassed:  false,
			wantErrSubs: []string{"no matching file satisfied"},
		},
		{
			name:  "must_not_match catches forbidden pattern",
			files: map[string]string{"foo_test.go": `import "github.com/stretchr/testify/assert"`},
			rules: []scenario.FileContentAssert{{
				Glob:         "*_test.go",
				MustNotMatch: []string{`testify`},
			}},
			wantPassed:  false,
			wantErrSubs: []string{"forbidden pattern present"},
		},
		{
			name:  "no file matches glob fails the rule",
			files: map[string]string{"main.go": ""},
			rules: []scenario.FileContentAssert{{
				Glob:      "*_test.go",
				MustMatch: []string{`anything`},
			}},
			wantPassed:  false,
			wantErrSubs: []string{"no mutated file matches glob"},
		},
		{
			name: "match_all_files: every file must pass",
			files: map[string]string{
				"a_test.go": `errors.Is(err, X)`,
				"b_test.go": `if err == X {}`, // missing errors.Is
			},
			rules: []scenario.FileContentAssert{{
				Glob:          "*_test.go",
				MustMatch:     []string{`errors\.Is`},
				MatchAllFiles: true,
			}},
			wantPassed:  false,
			wantErrSubs: []string{"b_test.go"},
		},
		{
			name: "at-least-one: any file passing is sufficient",
			files: map[string]string{
				"a_test.go": `errors.Is(err, X)`,
				"b_test.go": `if err == X {}`,
			},
			rules: []scenario.FileContentAssert{{
				Glob:      "*_test.go",
				MustMatch: []string{`errors\.Is`},
			}},
			wantPassed: true,
		},
		{
			name:  "invalid regex is a scenario-author error, not silent pass",
			files: map[string]string{"foo.go": ""},
			rules: []scenario.FileContentAssert{{
				Glob:      "*.go",
				MustMatch: []string{`[unclosed`},
			}},
			wantPassed:  false,
			wantErrSubs: []string{"invalid must_match regex"},
		},
		{
			name:  "empty rules slice is a no-op (no assertion = pass)",
			files: map[string]string{"foo.go": "anything"},
			rules: nil,
			wantPassed: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := mkResult(c.files)
			jdg, err := j.Judge(result, scenario.Scenario{Assert: scenario.AssertConfig{FileContent: c.rules}})
			if err != nil {
				t.Fatalf("Judge returned error: %v", err)
			}
			if jdg.Passed != c.wantPassed {
				t.Errorf("Passed = %v, want %v. Errors: %v", jdg.Passed, c.wantPassed, jdg.Errors)
			}
			for _, sub := range c.wantErrSubs {
				if !containsAny(jdg.Errors, sub) {
					t.Errorf("expected error containing %q in %v", sub, jdg.Errors)
				}
			}
		})
	}
}

func containsAny(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}
