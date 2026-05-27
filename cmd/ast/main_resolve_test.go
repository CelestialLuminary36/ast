package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScenario(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "id: " + id + "\ninput:\n  user_prompt: \"hi\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func writeMinimalSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte("id: sk\nname: SK\n"), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Follow the user's request carefully."), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
}

func TestResolveScenarios_ExplicitWins(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)
	explicit := filepath.Join(root, "custom")
	writeScenario(t, filepath.Join(explicit, "a.yaml"), "a")
	writeScenario(t, filepath.Join(root, "scenarios", "sk", "b.yaml"), "b")

	scs, src := resolveScenarios(skillDir, "sk", filepath.Join(root, "scenarios"), explicit)
	if src != sourceExplicit {
		t.Fatalf("source = %v, want sourceExplicit", src)
	}
	if len(scs) != 1 || scs[0].ID != "a" {
		t.Fatalf("got %+v", scs)
	}
}

func TestResolveScenarios_PerSkillBeatsFlat(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)
	scenariosDir := filepath.Join(root, "scenarios")
	writeScenario(t, filepath.Join(scenariosDir, "sk", "good.yaml"), "good")
	writeScenario(t, filepath.Join(scenariosDir, "flat.yaml"), "flat")

	scs, src := resolveScenarios(skillDir, "sk", scenariosDir, "")
	if src != sourcePerSkill {
		t.Fatalf("source = %v, want sourcePerSkill", src)
	}
	if len(scs) != 1 || scs[0].ID != "good" {
		t.Fatalf("got %+v", scs)
	}
}

func TestResolveScenarios_NestedIsDeprecatedFallback(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)
	writeScenario(t, filepath.Join(skillDir, "scenarios", "x.yaml"), "x")

	scs, src := resolveScenarios(skillDir, "sk", filepath.Join(root, "scenarios"), "")
	if src != sourceNestedDeprecated {
		t.Fatalf("source = %v, want sourceNestedDeprecated", src)
	}
	if len(scs) != 1 || scs[0].ID != "x" {
		t.Fatalf("got %+v", scs)
	}
}

func TestResolveScenarios_NoneFound(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)

	_, src := resolveScenarios(skillDir, "sk", filepath.Join(root, "scenarios"), "")
	if src != sourceNotFound {
		t.Fatalf("source = %v, want sourceNotFound", src)
	}
}

func TestValidateSkill_PollutionWarning(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)
	// Pollution: scenarios inside the skill package
	writeScenario(t, filepath.Join(skillDir, "scenarios", "p.yaml"), "p")
	writeScenario(t, filepath.Join(root, "scenarios", "sk", "ok.yaml"), "ok")

	issues, warns := validateSkill(skillDir, filepath.Join(root, "scenarios"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	joined := strings.Join(warns, "\n")
	if !strings.Contains(joined, "should live outside the skill") {
		t.Fatalf("expected pollution warning, got: %v", warns)
	}
}

func TestValidateSkill_NoScenariosWarns(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	writeMinimalSkill(t, skillDir)

	issues, warns := validateSkill(skillDir, filepath.Join(root, "scenarios"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if !strings.Contains(strings.Join(warns, "\n"), "no scenarios at") {
		t.Fatalf("expected missing-scenarios warning, got: %v", warns)
	}
}

func TestValidateSkill_MissingInstructionsIsError(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "sk")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// only skill.yaml, no instructions
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("id: sk\nname: SK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScenario(t, filepath.Join(root, "scenarios", "sk", "ok.yaml"), "ok")

	issues, _ := validateSkill(skillDir, filepath.Join(root, "scenarios"))
	if len(issues) == 0 || !strings.Contains(strings.Join(issues, "\n"), "no instructions found") {
		t.Fatalf("expected instructions error, got: %v", issues)
	}
}
