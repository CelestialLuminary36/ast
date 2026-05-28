package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromDir inspects dir, picks the most-specific format that matches,
// and normalises into a *Skill. The dispatch order is:
//
//  1. Anthropic package      — skill.yaml present
//  2. Cursor rules           — .cursor/rules/*.mdc or .cursorrules
//  3. AGENTS.md (Codex)      — AGENTS.md at the root
//  4. Frontmatter fallback   — any .md with YAML frontmatter
//
// Formats further down the list have fewer guarantees (no tool whitelist,
// no canonical id field), so callers should check skill.Format before
// asserting on tools or metadata.
func LoadFromDir(dir string) (*Skill, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	switch detectFormat(dir) {
	case FormatAnthropic:
		return loadAnthropic(dir)
	case FormatCursorRules:
		return loadCursor(dir)
	case FormatAgentsMD:
		return loadAgentsMD(dir)
	case FormatFrontmatter:
		return loadFrontmatter(dir)
	default:
		return nil, fmt.Errorf("no recognisable skill format in %s (looked for skill.yaml, .cursor/rules/*.mdc, .cursorrules, AGENTS.md, or any .md with frontmatter)", dir)
	}
}

func detectFormat(dir string) Format {
	if fileExists(filepath.Join(dir, "skill.yaml")) {
		return FormatAnthropic
	}
	if fileExists(filepath.Join(dir, ".cursorrules")) {
		return FormatCursorRules
	}
	if dirHasFiles(filepath.Join(dir, ".cursor", "rules"), ".mdc") {
		return FormatCursorRules
	}
	if fileExists(filepath.Join(dir, "AGENTS.md")) {
		return FormatAgentsMD
	}
	if dirHasFiles(filepath.Join(dir, ".agents"), ".md") {
		return FormatAgentsMD
	}
	if anyFileHasFrontmatter(dir) {
		return FormatFrontmatter
	}
	return ""
}

// ---------- Anthropic format ----------

func loadAnthropic(dir string) (*Skill, error) {
	s := &Skill{
		Path:   dir,
		Format: FormatAnthropic,
		Meta:   make(map[string]any),
	}

	skillYamlPath := filepath.Join(dir, "skill.yaml")
	if data, err := os.ReadFile(skillYamlPath); err == nil {
		var meta struct {
			ID          string         `yaml:"id"`
			Name        string         `yaml:"name"`
			Description string         `yaml:"description"`
			Version     string         `yaml:"version"`
			Extra       map[string]any `yaml:",inline"`
		}
		if err := yaml.Unmarshal(data, &meta); err == nil {
			s.ID = meta.ID
			s.Name = meta.Name
			if meta.Description != "" {
				s.Meta["description"] = meta.Description
			}
			if meta.Version != "" {
				s.Meta["version"] = meta.Version
			}
			for k, v := range meta.Extra {
				if k != "id" && k != "name" && k != "description" && k != "version" {
					s.Meta[k] = v
				}
			}
		}
	}

	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	if s.ID == "" {
		s.ID = s.Name
	}

	for _, fname := range []string{"instructions.md", "skill.md", "README.md", "instruction.txt", "instruction.md"} {
		p := filepath.Join(dir, fname)
		if data, err := os.ReadFile(p); err == nil {
			s.Instructions = string(data)
			break
		}
	}

	toolsDir := filepath.Join(dir, "tools")
	toolsInfo, toolsErr := os.Stat(toolsDir)
	if toolsErr == nil && toolsInfo.IsDir() {
		entries, err := os.ReadDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("read tools dir: %w", err)
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			p := filepath.Join(toolsDir, e.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", p, err)
			}
			var td ToolDef
			if err := json.Unmarshal(data, &td); err != nil {
				return nil, fmt.Errorf("parse %s: %w", p, err)
			}
			if td.Name == "" {
				return nil, fmt.Errorf("%s: tool definition is missing required field 'name'", p)
			}
			s.ToolDefs = append(s.ToolDefs, td)
			names = append(names, td.Name)
		}
		if len(names) > 0 {
			s.Meta["tools"] = names
		}
	}

	return s, nil
}

// ---------- Cursor rules ----------
//
// Two layouts:
//   .cursorrules            — single plain-text file (legacy)
//   .cursor/rules/*.mdc     — multiple files, each with YAML frontmatter
//
// .mdc frontmatter fields we care about:
//   description, globs (string or []string), alwaysApply (bool)
// Everything else lands in Meta untouched. Cursor has no concept of a tool
// whitelist, so ToolDefs stays empty — runner default (all builtins) applies.

func loadCursor(dir string) (*Skill, error) {
	s := &Skill{
		Path:   dir,
		Format: FormatCursorRules,
		Name:   filepath.Base(dir),
		Meta:   make(map[string]any),
	}
	s.ID = s.Name

	// Legacy single-file form first.
	if data, err := os.ReadFile(filepath.Join(dir, ".cursorrules")); err == nil {
		s.Instructions = string(data)
		s.Meta["source"] = ".cursorrules"
		return s, nil
	}

	rulesDir := filepath.Join(dir, ".cursor", "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("read cursor rules dir: %w", err)
	}

	// Concatenate every .mdc file in lexicographic order so the result is
	// reproducible across runs. Frontmatter from the first file populates
	// metadata; later files contribute body text only.
	var sources []string
	var bodies []string
	firstMeta := true
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".mdc") {
			continue
		}
		p := filepath.Join(rulesDir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		fm, body := splitFrontmatter(data)
		sources = append(sources, e.Name())
		bodies = append(bodies, fmt.Sprintf("# %s\n\n%s", e.Name(), strings.TrimSpace(body)))

		if firstMeta && len(fm) > 0 {
			var meta map[string]any
			if err := yaml.Unmarshal(fm, &meta); err == nil {
				for k, v := range meta {
					s.Meta[k] = v
				}
				if d, ok := meta["description"].(string); ok && d != "" {
					s.Meta["description"] = d
				}
			}
			firstMeta = false
		}
	}
	if len(bodies) == 0 {
		return nil, fmt.Errorf("cursor rules dir has no .mdc files")
	}
	s.Instructions = strings.Join(bodies, "\n\n---\n\n")
	s.Meta["sources"] = sources
	return s, nil
}

// ---------- AGENTS.md (Codex / generic) ----------
//
// AGENTS.md is just markdown — we treat the whole file as instructions.
// .agents/*.md is the multi-file variant (concatenated, like cursor rules).

func loadAgentsMD(dir string) (*Skill, error) {
	s := &Skill{
		Path:   dir,
		Format: FormatAgentsMD,
		Name:   filepath.Base(dir),
		Meta:   make(map[string]any),
	}
	s.ID = s.Name

	if data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md")); err == nil {
		fm, body := splitFrontmatter(data)
		if len(fm) > 0 {
			var meta map[string]any
			if yaml.Unmarshal(fm, &meta) == nil {
				for k, v := range meta {
					s.Meta[k] = v
				}
				if n, ok := meta["name"].(string); ok && n != "" {
					s.Name = n
				}
				if id, ok := meta["id"].(string); ok && id != "" {
					s.ID = id
				}
			}
		}
		s.Instructions = strings.TrimSpace(body)
		s.Meta["source"] = "AGENTS.md"
		return s, nil
	}

	agentsDir := filepath.Join(dir, ".agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("read agents dir: %w", err)
	}
	var sources []string
	var bodies []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		p := filepath.Join(agentsDir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		_, body := splitFrontmatter(data)
		sources = append(sources, e.Name())
		bodies = append(bodies, fmt.Sprintf("# %s\n\n%s", e.Name(), strings.TrimSpace(body)))
	}
	if len(bodies) == 0 {
		return nil, fmt.Errorf(".agents/ has no .md files")
	}
	s.Instructions = strings.Join(bodies, "\n\n---\n\n")
	s.Meta["sources"] = sources
	return s, nil
}

// ---------- Generic frontmatter fallback ----------
//
// Loads the first .md file (lexicographically) in dir that has YAML
// frontmatter. The frontmatter populates ID/Name/Meta; the body is the
// instructions. This catches custom layouts users invent that fit the
// "markdown-with-metadata" mental model.

func loadFrontmatter(dir string) (*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		fm, body := splitFrontmatter(data)
		if len(fm) == 0 {
			continue
		}
		var meta map[string]any
		if err := yaml.Unmarshal(fm, &meta); err != nil {
			continue
		}
		s := &Skill{
			Path:         dir,
			Format:       FormatFrontmatter,
			Instructions: strings.TrimSpace(body),
			Meta:         meta,
		}
		if n, ok := meta["name"].(string); ok && n != "" {
			s.Name = n
		} else {
			s.Name = filepath.Base(dir)
		}
		if id, ok := meta["id"].(string); ok && id != "" {
			s.ID = id
		} else {
			s.ID = s.Name
		}
		s.Meta["source"] = e.Name()
		return s, nil
	}
	return nil, fmt.Errorf("no .md file with YAML frontmatter found in %s", dir)
}

// ---------- helpers ----------

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirHasFiles(p, ext string) bool {
	entries, err := os.ReadDir(p)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ext) {
			return true
		}
	}
	return false
}

func anyFileHasFrontmatter(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fm, _ := splitFrontmatter(data)
		if len(fm) > 0 {
			return true
		}
	}
	return false
}

// splitFrontmatter accepts file bytes and returns (frontmatter, body).
// Frontmatter must start at byte 0 with "---" followed by a newline, and
// terminate with another "---" on its own line. If absent, returns empty
// frontmatter and the original bytes as body. Both CRLF and LF line
// endings are accepted; the frontmatter result is normalised to LF for
// the YAML parser.
func splitFrontmatter(data []byte) ([]byte, string) {
	// Strip a UTF-8 BOM if present — common on Windows-edited files.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, s
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, s
	}
	fm := rest[:end]
	// Body starts after the closing fence and its trailing newline (or EOF).
	bodyStart := end + len("\n---")
	if bodyStart < len(rest) && rest[bodyStart] == '\n' {
		bodyStart++
	}
	body := rest[bodyStart:]
	return []byte(fm), body
}
