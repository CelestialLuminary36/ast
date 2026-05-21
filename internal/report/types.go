package report

import (
	"time"
)

type Entry struct {
	ScenarioID   string        `json:"scenario_id"`
	Passed       bool          `json:"passed"`
	Output       string        `json:"output"`
	ExecutedCmds []string      `json:"executed_cmds,omitempty"`
	MutatedFiles []string      `json:"mutated_files,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
	Duration     time.Duration `json:"duration"`
}

type Report struct {
	Project   string    `json:"project"`
	SkillName string    `json:"skill_name"`
	SkillPath string    `json:"skill_path"`
	Timestamp time.Time `json:"timestamp"`
	Passed    int       `json:"passed"`
	Failed    int       `json:"failed"`
	Entries   []Entry   `json:"entries"`
}
