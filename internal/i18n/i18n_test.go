package i18n

import (
	"testing"
)

func TestTEnglish(t *testing.T) {
	Set(EN)
	got := T(MsgInitGitSandbox)
	want := "Initializing isolated Git sandbox ... "
	if got != want {
		t.Errorf("T(MsgInitGitSandbox) = %q, want %q", got, want)
	}
}

func TestTChinese(t *testing.T) {
	Set(ZH)
	got := T(MsgInitGitSandbox)
	want := "初始化 Git 隔离沙盒 ... "
	if got != want {
		t.Errorf("T(MsgInitGitSandbox) = %q, want %q", got, want)
	}
}

func TestTFormat(t *testing.T) {
	Set(EN)
	got := T(MsgFailedRules, 3)
	want := " (3 rule(s) failed)\n"
	if got != want {
		t.Errorf("T(MsgFailedRules, 3) = %q, want %q", got, want)
	}

	Set(ZH)
	got = T(MsgFailedRules, 5)
	want = " (5 条规则未通过)\n"
	if got != want {
		t.Errorf("T(MsgFailedRules, 5) = %q, want %q", got, want)
	}
}

func TestTFallback(t *testing.T) {
	Set(EN)
	got := T("nonexistent_key")
	if got != "nonexistent_key" {
		t.Errorf("expected key as fallback, got %q", got)
	}
}

func TestAllKeysHaveTranslations(t *testing.T) {
	keys := []string{
		MsgInitGitSandbox, MsgCallAgent, MsgDuration, MsgRuleAudit,
		MsgFailedRules, MsgGenReport, MsgRunScenario, MsgFileMutations,
		MsgExecutedCmds, MsgResultSummary, MsgExampleName, MsgExampleDesc,
	}
	for _, k := range keys {
		if enMessages[k] == "" {
			t.Errorf("enMessages missing key %q", k)
		}
		if zhMessages[k] == "" {
			t.Errorf("zhMessages missing key %q", k)
		}
	}
}
