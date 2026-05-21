$ErrorActionPreference = "Stop"
$ProjectDir = Split-Path -Parent $PSScriptRoot
$Ast = "$ProjectDir\ast.exe"

Set-Location $ProjectDir

Write-Host "=== Building ast CLI ===" -ForegroundColor Cyan
if (Test-Path "$Ast") { Remove-Item "$Ast" }
go build -o ast.exe .\cmd\ast

Write-Host ""
Write-Host "=== Test 1: init command ===" -ForegroundColor Cyan
if (Test-Path "test_workspace") { Remove-Item -Recurse -Force "test_workspace" }
New-Item -ItemType Directory -Name "test_workspace" | Out-Null
Set-Location "test_workspace"

& "$Ast" init

if (-not (Test-Path "ast.yaml")) { throw "FAIL: ast.yaml not created" }
if (-not (Test-Path "scenarios")) { throw "FAIL: scenarios/ not created" }
if (-not (Test-Path "reports")) { throw "FAIL: reports/ not created" }
Write-Host "PASS: init created expected files" -ForegroundColor Green

Write-Host ""
Write-Host "=== Test 2: create test skill ===" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path "skills\go-reviewer" | Out-Null

@"
id: go-reviewer
name: Go Reviewer
version: "1.0.0"
description: Fix Go panics safely
"@ | Set-Content -Path "skills\go-reviewer\skill.yaml" -Encoding UTF8

@"
You are a Go expert. When fixing panics:
1. Add nil checks
2. Run `go test ./...` before declaring done
3. NEVER modify vendor/ or go.mod
"@ | Set-Content -Path "skills\go-reviewer\instructions.md" -Encoding UTF8

Write-Host "PASS: skill created" -ForegroundColor Green

function Run-Test($Name, $ScenarioContent, $SkillDir, $ShouldContain, $LogFile) {
    $ScenarioContent | Set-Content -Path "scenarios\$Name.yaml" -Encoding UTF8
    & "$Ast" test $SkillDir > $LogFile 2>&1
    $log = Get-Content $LogFile -Raw
    if ($log -match $ShouldContain) {
        Write-Host "PASS: $Name" -ForegroundColor Green
    } else {
        Write-Host "FAIL: $Name" -ForegroundColor Red
        Write-Host $log
        throw "Test failed"
    }
    Remove-Item "scenarios\$Name.yaml" -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "=== Test 3: basic test (should PASS) ===" -ForegroundColor Cyan
& "$Ast" test .\skills\go-reviewer > test3.log 2>&1
if ((Get-Content test3.log -Raw) -match "PASSED") {
    Write-Host "PASS: basic scenario passed" -ForegroundColor Green
} else {
    throw "FAIL: basic scenario did not pass"
}

Write-Host ""
Write-Host "=== Test 4: output_text assertion (should FAIL) ===" -ForegroundColor Cyan
$scenario = @"
id: test-output-fail
name: "输出文本失败测试"
input:
  user_prompt: "Fix it"
assert:
  output_text:
    must_include:
      - "不可能出现的文本XYZ"
"@
Run-Test "test-output-fail" $scenario ".\skills\go-reviewer" "output missing required text" "test4.log"

Write-Host ""
Write-Host "=== Test 5: command_execution must_not_have (should FAIL) ===" -ForegroundColor Cyan
$scenario = @"
id: test-cmd-fail
name: "命令审计失败测试"
input:
  user_prompt: "Fix it"
assert:
  command_execution:
    must_not_have:
      - contains: "go test"
"@
Run-Test "test-cmd-fail" $scenario ".\skills\go-reviewer" "forbidden command detected" "test5.log"

Write-Host ""
Write-Host "=== Test 6: command_execution must_have (should PASS) ===" -ForegroundColor Cyan
$scenario = @"
id: test-cmd-pass
name: "命令审计通过测试"
input:
  user_prompt: "Fix it"
assert:
  command_execution:
    must_have:
      - contains: "go test ./..."
        min_count: 1
"@
Run-Test "test-cmd-pass" $scenario ".\skills\go-reviewer" "PASSED" "test6.log"

Write-Host ""
Write-Host "=== Test 7: file_mutations with fixture ===" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path "fixtures\go-service\internal\handler" | Out-Null
"package handler" | Set-Content -Path "fixtures\go-service\internal\handler\user.go"
"module demo" | Set-Content -Path "fixtures\go-service\go.mod"

$scenario = @"
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
"@
Run-Test "test-files" $scenario ".\skills\go-reviewer" "PASSED" "test7.log"

Write-Host ""
Write-Host "=== Test 8: report command ===" -ForegroundColor Cyan
$LatestJson = Get-ChildItem "reports\report-*.json" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
& "$Ast" report $LatestJson.FullName > test8.log 2>&1
if ((Get-Content test8.log -Raw) -match "测试结果摘要") {
    Write-Host "PASS: report command works" -ForegroundColor Green
} else {
    throw "FAIL: report command did not work"
}

Write-Host ""
Write-Host "=== Test 9: Markdown report generated ===" -ForegroundColor Cyan
$LatestMd = Get-ChildItem "reports\report-*.md" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ((Get-Content $LatestMd.FullName -Raw) -match "Skill Check Report") {
    Write-Host "PASS: Markdown report generated correctly" -ForegroundColor Green
} else {
    throw "FAIL: Markdown report missing"
}

Set-Location $ProjectDir
Remove-Item -Recurse -Force "test_workspace"

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "All tests passed!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
