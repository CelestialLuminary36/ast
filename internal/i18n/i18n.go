package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Lang represents a language code.
type Lang string

const (
	EN Lang = "en"
	ZH Lang = "zh"
)

// Message keys for all user-visible strings.
const (
	MsgInitGitSandbox = "init_git_sandbox"
	MsgCallAgent      = "call_agent"
	MsgDuration       = "duration"
	MsgRuleAudit      = "rule_audit"
	MsgFailedRules    = "failed_rules"
	MsgGenReport      = "gen_report"
	MsgRunScenario    = "run_scenario"
	MsgFileMutations  = "file_mutations"
	MsgExecutedCmds   = "executed_commands"
	MsgResultSummary  = "result_summary"
	MsgExampleName    = "example_name"
	MsgExampleDesc    = "example_desc"
)

var current Lang = EN

func init() {
	detect()
}

func detect() {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(env)
		if strings.HasPrefix(strings.ToLower(v), "zh") {
			current = ZH
			return
		}
	}
}

// Set switches the active language. Call early, before any T() lookups.
func Set(l Lang) { current = l }

// Get returns the current language.
func Get() Lang { return current }

// T returns the translation for key, formatted with args (passed to fmt.Sprintf).
// Falls back to the English message, then the raw key if no translation exists.
func T(key string, args ...interface{}) string {
	var tmpl string
	if current == ZH {
		tmpl = zhMessages[key]
	}
	if tmpl == "" {
		tmpl = enMessages[key]
	}
	if tmpl == "" {
		tmpl = key
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

var enMessages = map[string]string{
	MsgInitGitSandbox: "Initializing isolated Git sandbox ... ",
	MsgCallAgent:      "Calling agent ... ",
	MsgDuration:       " (took %s)\n",
	MsgRuleAudit:      "Running rule audit ... ",
	MsgFailedRules:    " (%d rule(s) failed)\n",
	MsgGenReport:      "Generating test report ...",
	MsgRunScenario:    "Scenario [%d/%d]: %s ... %s\n",
	MsgFileMutations:  "  ├── File mutations: %s\n",
	MsgExecutedCmds:   "  ├── Executed commands: %s\n",
	MsgResultSummary:  "Summary: TOTAL: %d | PASSED: %d | FAILED: %d\n",
	MsgExampleName:    "Example Scenario",
	MsgExampleDesc:    "Verify agent can generate a React button component without touching forbidden files",
}

var zhMessages = map[string]string{
	MsgInitGitSandbox: "初始化 Git 隔离沙盒 ... ",
	MsgCallAgent:      "调用 Agent ... ",
	MsgDuration:       " (耗时 %s)\n",
	MsgRuleAudit:      "规则审计 ... ",
	MsgFailedRules:    " (%d 条规则未通过)\n",
	MsgGenReport:      "生成测试报告 ...",
	MsgRunScenario:    "运行场景 [%d/%d]: %s ... %s\n",
	MsgFileMutations:  "  ├── 文件变动: %s\n",
	MsgExecutedCmds:   "  ├── 执行命令: %s\n",
	MsgResultSummary:  "测试结果摘要: TOTAL: %d | PASSED: %d | FAILED: %d\n",
	MsgExampleName:    "示例场景",
	MsgExampleDesc:    "验证 Agent 是否能生成 React 按钮组件且不触碰禁止文件",
}
