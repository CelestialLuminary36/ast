package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hhy/ast/internal/scenario"
	"github.com/hhy/ast/internal/skill"
)

const helpList = `ast list - Enumerate skills in a directory and show their status.

Usage:
  ast list [--dir=DIR]

Flags:
  --dir=DIR   Directory to scan. Default: ./skills

For each immediate subdirectory of DIR, attempts to load it as a skill
and prints a one-line summary with id, detected format, number of
scenarios that would run with 'ast test', and a status flag (OK,
WARN, or ERROR). Directories that don't parse as a skill are reported
as ERROR rather than silently skipped, so typos or half-finished work
remain visible.

Status is a quick health glance, not a full audit — use 'ast validate
<skill-dir>' for the detailed lint output on a specific skill.`

type listRow struct {
	dirName   string
	id        string
	format    string
	scenarios int
	status    string
	detail    string
}

func cmdList(args []string) error {
	if wantsHelp(args) {
		fmt.Println(helpList)
		return nil
	}

	dir := "./skills"
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--dir="); ok {
			dir = strings.TrimSpace(v)
		}
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("scan %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	cfg := loadConfigOrDefault()

	rows := make([]listRow, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rows = append(rows, inspectSkillDir(filepath.Join(dir, e.Name()), e.Name(), cfg.ScenariosDir))
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].dirName < rows[j].dirName })

	if len(rows) == 0 {
		fmt.Printf("No skill directories found under %s\n", dir)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SKILL\tID\tFORMAT\tSCENARIOS\tSTATUS")
	for _, r := range rows {
		detail := ""
		if r.detail != "" {
			detail = "  " + r.detail
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s%s\n", r.dirName, r.id, r.format, r.scenarios, r.status, detail)
	}
	return w.Flush()
}

// inspectSkillDir is the per-row work for `ast list`. Pulled out so it can be
// unit-tested without spinning up the whole CLI dispatcher. Never returns an
// error — failures surface as the row's status field instead, because the
// caller is enumerating and a single broken skill should not abort the scan.
func inspectSkillDir(path, dirName, scenariosDir string) listRow {
	row := listRow{dirName: dirName, id: "-", format: "-"}

	sk, err := skill.LoadFromDir(path)
	if err != nil {
		row.status = "ERROR"
		row.detail = err.Error()
		return row
	}
	row.id = sk.ID
	row.format = string(sk.Format)

	// Scenario count: prefer per-skill dir, fall back to flat layout exactly
	// like `ast test` does. Anything else (e.g. nested-in-skill) is a
	// deprecated layout and we don't count it here — `ast validate` warns.
	row.scenarios = countScenarios(filepath.Join(scenariosDir, sk.ID))
	if row.scenarios == 0 {
		row.scenarios = countScenarios(scenariosDir)
	}

	issues, warns := validateSkill(path, scenariosDir)
	switch {
	case len(issues) > 0:
		row.status = "ERROR"
		row.detail = fmt.Sprintf("%d issue(s)", len(issues))
	case len(warns) > 0:
		row.status = "WARN"
		row.detail = fmt.Sprintf("%d warning(s)", len(warns))
	default:
		row.status = "OK"
	}
	return row
}

func countScenarios(dir string) int {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return 0
	}
	scs, err := scenario.LoadFromDir(dir)
	if err != nil {
		return 0
	}
	return len(scs)
}
