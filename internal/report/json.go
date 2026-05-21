package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Save(path string, r *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Load(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Report) AddEntry(e Entry) {
	r.Entries = append(r.Entries, e)
	if e.Passed {
		r.Passed++
	} else {
		r.Failed++
	}
}

func (r *Report) PrintConsole() {
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Project: %s\n", r.Project)
	fmt.Printf("Skill:   %s (%s)\n", r.SkillName, r.SkillPath)
	fmt.Printf("Time:    %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Println()

	total := len(r.Entries)
	for i, e := range r.Entries {
		status := "PASSED"
		if !e.Passed {
			status = "FAILED"
		}
		fmt.Printf("运行场景 [%d/%d]: %s ... %s\n", i+1, total, e.ScenarioID, status)
		if len(e.MutatedFiles) > 0 {
			fmt.Printf("  ├── 文件变动: %s\n", strings.Join(e.MutatedFiles, ", "))
		}
		if len(e.ExecutedCmds) > 0 {
			fmt.Printf("  ├── 执行命令: %s\n", strings.Join(e.ExecutedCmds, ", "))
		}
		if !e.Passed {
			for _, err := range e.Errors {
				fmt.Printf("  └── ERROR: %s\n", err)
			}
		}
	}

	fmt.Println()
	fmt.Printf("测试结果摘要: TOTAL: %d | PASSED: %d | FAILED: %d\n", total, r.Passed, r.Failed)
	fmt.Println("--------------------------------------------------")
}

func (r *Report) SaveMarkdown(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# Skill Check Report\n\n")
	b.WriteString(fmt.Sprintf("- **Project:** %s\n", r.Project))
	b.WriteString(fmt.Sprintf("- **Skill:** %s (%s)\n", r.SkillName, r.SkillPath))
	b.WriteString(fmt.Sprintf("- **Time:** %s\n", r.Timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Result:** TOTAL: %d | PASSED: %d | FAILED: %d\n\n", len(r.Entries), r.Passed, r.Failed))

	b.WriteString("## Scenarios\n\n")
	for _, e := range r.Entries {
		status := "PASS"
		if !e.Passed {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("### %s — %s\n\n", e.ScenarioID, status))
		b.WriteString(fmt.Sprintf("- **Duration:** %s\n", e.Duration))
		if len(e.MutatedFiles) > 0 {
			b.WriteString(fmt.Sprintf("- **Mutated Files:** %s\n", strings.Join(e.MutatedFiles, ", ")))
		}
		if len(e.ExecutedCmds) > 0 {
			b.WriteString(fmt.Sprintf("- **Executed Commands:** %s\n", strings.Join(e.ExecutedCmds, ", ")))
		}
		if !e.Passed {
			b.WriteString("- **Errors:**\n")
			for _, err := range e.Errors {
				b.WriteString(fmt.Sprintf("  - %s\n", err))
			}
		}
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}
