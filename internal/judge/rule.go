package judge

import (
	"fmt"
	"path"
	"strings"

	"github.com/CelestialLuminary36/agent-skill-test/internal/runner"
	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
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

func (j *Judgement) append(other *Judgement) {
	if !other.Passed {
		j.Passed = false
	}
	j.Errors = append(j.Errors, other.Errors...)
}

// matchGlob performs simple glob matching supporting * and **.
// ** matches any number of directory segments.
func matchGlob(pattern, s string) bool {
	// Normalize separators
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	s = strings.ReplaceAll(s, "\\", "/")

	// Handle ** patterns by converting to a simple prefix/suffix check
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if strings.HasPrefix(s, prefix) && strings.HasSuffix(s, suffix) {
				// Ensure the middle part is valid (no prefix/suffix overlap issues for simple cases)
				return true
			}
		}
		// Fallback: treat ** as * for path.Match
		pattern = strings.ReplaceAll(pattern, "**", "*")
	}

	matched, _ := path.Match(pattern, s)
	return matched
}
