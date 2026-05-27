package judge

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hhy/ast/internal/runner"
	"github.com/hhy/ast/internal/scenario"
)

type RuleJudge struct{}

func NewRuleJudge() *RuleJudge {
	return &RuleJudge{}
}

func (j *RuleJudge) Judge(result *runner.RunResult, sc scenario.Scenario) (*Judgement, error) {
	jdg := &Judgement{Passed: true}

	// 1. File mutation audit
	jdg.append(j.auditFileMutations(result.MutatedFiles, sc.Assert.FileMutations))

	// 2. Command execution audit
	jdg.append(j.auditCommands(result.ExecutedCmds, sc.Assert.CommandExecution))

	// 3. Output text audit
	jdg.append(j.auditOutputText(result.Output, sc.Assert.OutputText))

	// 4. File content audit (regex over post-run contents of mutated files)
	jdg.append(j.auditFileContent(result.MutatedFiles, result.FileContents, sc.Assert.FileContent))

	return jdg, nil
}

func (j *RuleJudge) auditFileMutations(files []string, assert scenario.FileMutationAssert) *Judgement {
	jdg := &Judgement{Passed: true}

	for _, f := range files {
		// Check forbidden patterns
		for _, pattern := range assert.Forbidden {
			if matchGlob(pattern, f) {
				jdg.Passed = false
				jdg.Errors = append(jdg.Errors, fmt.Sprintf("forbidden file modified: %s (matched %q)", f, pattern))
			}
		}
	}

	// If allowed list is specified, every mutated file must match at least one allowed pattern
	if len(assert.Allowed) > 0 {
		for _, f := range files {
			matched := false
			for _, pattern := range assert.Allowed {
				if matchGlob(pattern, f) {
					matched = true
					break
				}
			}
			if !matched {
				jdg.Passed = false
				jdg.Errors = append(jdg.Errors, fmt.Sprintf("file %s not in allowed mutation list", f))
			}
		}
	}

	return jdg
}

func (j *RuleJudge) auditCommands(cmds []string, assert scenario.CommandAssert) *Judgement {
	jdg := &Judgement{Passed: true}

	// Must have rules
	for _, rule := range assert.MustHave {
		count := 0
		for _, cmd := range cmds {
			if strings.Contains(cmd, rule.Contains) {
				count++
			}
		}
		min := rule.MinCount
		if min <= 0 {
			min = 1
		}
		if count < min {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("expected command containing %q at least %d time(s), found %d", rule.Contains, min, count))
		}
	}

	// Must not have rules
	for _, rule := range assert.MustNotHave {
		for _, cmd := range cmds {
			if strings.Contains(cmd, rule.Contains) {
				jdg.Passed = false
				jdg.Errors = append(jdg.Errors, fmt.Sprintf("forbidden command detected: %q", cmd))
				break
			}
		}
	}

	return jdg
}

func (j *RuleJudge) auditOutputText(output string, assert scenario.TextAssert) *Judgement {
	jdg := &Judgement{Passed: true}

	for _, phrase := range assert.MustInclude {
		if !strings.Contains(output, phrase) {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("output missing required text: %q", phrase))
		}
	}

	for _, phrase := range assert.MustNotInclude {
		if strings.Contains(output, phrase) {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("output contains forbidden text: %q", phrase))
		}
	}

	return jdg
}

func (j *RuleJudge) auditFileContent(mutated []string, contents map[string]string, rules []scenario.FileContentAssert) *Judgement {
	jdg := &Judgement{Passed: true}

	for ruleIdx, rule := range rules {
		// Find mutated files matching this rule's glob. We restrict to mutated
		// files (not the whole workspace) because file_content asserts what the
		// agent *changed* — unchanged files were already correct.
		var matchingFiles []string
		for _, f := range mutated {
			if matchGlob(rule.Glob, f) {
				matchingFiles = append(matchingFiles, f)
			}
		}
		if len(matchingFiles) == 0 {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d: no mutated file matches glob %q", ruleIdx+1, rule.Glob))
			continue
		}

		// Compile regexes once per rule. A compile failure is a scenario-author
		// bug, not an agent failure — surface it loudly.
		mustMatch, err := compileRegexes(rule.MustMatch)
		if err != nil {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d: invalid must_match regex: %v", ruleIdx+1, err))
			continue
		}
		mustNotMatch, err := compileRegexes(rule.MustNotMatch)
		if err != nil {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d: invalid must_not_match regex: %v", ruleIdx+1, err))
			continue
		}

		fileSatisfies := func(content string) (bool, string) {
			for i, re := range mustMatch {
				if !re.MatchString(content) {
					return false, fmt.Sprintf("missing required pattern %q", rule.MustMatch[i])
				}
			}
			for i, re := range mustNotMatch {
				if re.MatchString(content) {
					return false, fmt.Sprintf("forbidden pattern present %q", rule.MustNotMatch[i])
				}
			}
			return true, ""
		}

		if rule.MatchAllFiles {
			for _, f := range matchingFiles {
				content, ok := contents[f]
				if !ok {
					jdg.Passed = false
					jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d: %s has no captured content (binary or deleted?)", ruleIdx+1, f))
					continue
				}
				if pass, reason := fileSatisfies(content); !pass {
					jdg.Passed = false
					jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d: %s — %s", ruleIdx+1, f, reason))
				}
			}
			continue
		}

		// Default: at least one matching file must satisfy the rule.
		anyPassed := false
		var reasons []string
		for _, f := range matchingFiles {
			content, ok := contents[f]
			if !ok {
				reasons = append(reasons, fmt.Sprintf("%s: no captured content", f))
				continue
			}
			if pass, reason := fileSatisfies(content); pass {
				anyPassed = true
				break
			} else {
				reasons = append(reasons, fmt.Sprintf("%s: %s", f, reason))
			}
		}
		if !anyPassed {
			jdg.Passed = false
			jdg.Errors = append(jdg.Errors, fmt.Sprintf("file_content rule #%d (glob %q): no matching file satisfied the rule. tried: %s", ruleIdx+1, rule.Glob, strings.Join(reasons, "; ")))
		}
	}

	return jdg
}

func compileRegexes(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

func (j *Judgement) append(other *Judgement) {
	if !other.Passed {
		j.Passed = false
	}
	j.Errors = append(j.Errors, other.Errors...)
}

// matchGlob performs glob matching with full support for ** segment expansion,
// delegating to doublestar/v4. Slashes are normalized to forward slashes so
// Windows-style paths in `git status` output match POSIX-style patterns in
// scenario YAML.
//
// Uses doublestar.Match (always '/' separator), not PathMatch (OS-specific
// separator). On Windows PathMatch treats '/' as a literal character which
// makes `*.go` over-match `src/main.go` and `**` collapse incorrectly.
func matchGlob(pattern, s string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	s = strings.ReplaceAll(s, "\\", "/")
	ok, _ := doublestar.Match(pattern, s)
	return ok
}
