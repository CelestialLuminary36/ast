#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
AST="$PROJECT_DIR/ast"

cd "$PROJECT_DIR"

echo "=== Building ast CLI ==="
go build -o ast ./cmd/ast

echo ""
echo "=== Test 1: init command ==="
rm -rf test_workspace
mkdir test_workspace
cd test_workspace
"$AST" init

if [[ ! -f ast.yaml ]]; then
    echo "FAIL: ast.yaml not created"
    exit 1
fi
if [[ ! -d scenarios ]]; then
    echo "FAIL: scenarios/ not created"
    exit 1
fi
if [[ ! -d reports ]]; then
    echo "FAIL: reports/ not created"
    exit 1
fi
echo "PASS: init created expected files"

echo ""
echo "=== Test 2: create test skill ==="
mkdir -p skills/go-reviewer

cat > skills/go-reviewer/skill.yaml <<'SKILL'
id: go-reviewer
name: Go Reviewer
version: "1.0.0"
description: Fix Go panics safely
SKILL

cat > skills/go-reviewer/instructions.md <<'INST'
You are a Go expert. When fixing panics:
1. Add nil checks
2. Run `go test ./...` before declaring done
3. NEVER modify vendor/ or go.mod
INST

echo "PASS: skill created"

echo ""
echo "=== Test 3: basic test (should PASS) ==="
"$AST" test ./skills/go-reviewer > test3.log 2>&1
if grep -q "PASSED" test3.log; then
    echo "PASS: basic scenario passed"
else
    echo "FAIL: basic scenario did not pass"
    cat test3.log
    exit 1
fi

echo ""
echo "=== Test 4: output_text assertion (should FAIL) ==="
cat > scenarios/test-output-fail.yaml <<'YAML'
id: test-output-fail
name: "输出文本失败测试"
input:
  user_prompt: "Fix it"
assert:
  output_text:
    must_include:
      - "不可能出现的文本XYZ"
YAML

if ! "$AST" test ./skills/go-reviewer > test4.log 2>&1; then
    : # test command itself should not exit with error, just report FAIL
fi

if grep -q "output missing required text" test4.log; then
    echo "PASS: output_text correctly detected missing text"
else
    echo "FAIL: output_text did not catch missing text"
    cat test4.log
    exit 1
fi

rm scenarios/test-output-fail.yaml

echo ""
echo "=== Test 5: command_execution must_not_have (should FAIL) ==="
cat > scenarios/test-cmd-fail.yaml <<'YAML'
id: test-cmd-fail
name: "命令审计失败测试"
input:
  user_prompt: "Fix it"
assert:
  command_execution:
    must_not_have:
      - contains: "go test"
YAML

if ! "$AST" test ./skills/go-reviewer > test5.log 2>&1; then
    :
fi

if grep -q "forbidden command detected" test5.log; then
    echo "PASS: command_execution correctly detected forbidden command"
else
    echo "FAIL: command_execution did not catch forbidden command"
    cat test5.log
    exit 1
fi

rm scenarios/test-cmd-fail.yaml

echo ""
echo "=== Test 6: command_execution must_have (should PASS) ==="
cat > scenarios/test-cmd-pass.yaml <<'YAML'
id: test-cmd-pass
name: "命令审计通过测试"
input:
  user_prompt: "Fix it"
assert:
  command_execution:
    must_have:
      - contains: "go test ./..."
        min_count: 1
YAML

"$AST" test ./skills/go-reviewer > test6.log 2>&1
if grep -q "test-cmd-pass.*PASSED" test6.log || grep -q "test-cmd-pass" test6.log && ! grep -q "test-cmd-pass.*FAILED" test6.log; then
    echo "PASS: command_execution must_have works"
else
    echo "FAIL: command_execution must_have did not work"
    cat test6.log
    exit 1
fi

rm scenarios/test-cmd-pass.yaml

echo ""
echo "=== Test 7: file_mutations with fixture ==="
mkdir -p fixtures/go-service/internal/handler
echo "package handler" > fixtures/go-service/internal/handler/user.go
echo "module demo" > fixtures/go-service/go.mod

cat > scenarios/test-files.yaml <<'YAML'
id: test-files
name: "文件变动测试"
environment:
  fixture_dir: "./fixtures/go-service"
  init_script: "git init && git add . && git commit -m init"
input:
  user_prompt: "Fix the handler"
assert:
  file_mutations:
    allowed:
      - "internal/**"
    forbidden:
      - "go.mod"
YAML

"$AST" test ./skills/go-reviewer > test7.log 2>&1
if grep -q "test-files.*PASSED" test7.log || ! grep -q "test-files.*FAILED" test7.log; then
    echo "PASS: file_mutations with fixture works"
else
    echo "FAIL: file_mutations did not work"
    cat test7.log
    exit 1
fi

rm scenarios/test-files.yaml

echo ""
echo "=== Test 8: report command ==="
LATEST_JSON=$(ls -t reports/report-*.json | head -1)
"$AST" report "$LATEST_JSON" > test8.log 2>&1
if grep -q "测试结果摘要" test8.log; then
    echo "PASS: report command works"
else
    echo "FAIL: report command did not work"
    cat test8.log
    exit 1
fi

echo ""
echo "=== Test 9: Markdown report generated ==="
LATEST_MD=$(ls -t reports/report-*.md | head -1)
if grep -q "Skill Check Report" "$LATEST_MD"; then
    echo "PASS: Markdown report generated correctly"
else
    echo "FAIL: Markdown report missing"
    exit 1
fi

cd "$PROJECT_DIR"
rm -rf test_workspace

echo ""
echo "========================================"
echo "All tests passed!"
echo "========================================"
