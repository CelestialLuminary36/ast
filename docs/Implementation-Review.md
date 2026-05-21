# Skill-Check 实现走查：PRD 对齐度与偏离分析

> 复核对象：`docs/Skill-Check.md`（PRD）与当前代码库实现
> 复核目的：判断实现是否守住了"测 skill 约束力，过测即可上线"的本意

---

## 一、用户原始意图回顾

> 做一个命令行工具，用于测试 agent 的 skill 是否能按预期执行、不偏离 skill 的规则与约束；测试成功，则视为该 skill 可以上线的版本。

简言之：**测的是 skill 的"约束力"，不是模型的"智商"**。

---

## 二、PRD 与原始意图的对齐情况

[docs/Skill-Check.md](Skill-Check.md) 的定位（"CI for AI Agent Skills" / Behavior Regression Testing）与四层架构（Scenario DSL → Skill Package → Runner → Oracle Judge）**完全对得上**用户的意图。PRD 本身没有跑偏。

---

## 三、代码实现 vs PRD：对得上的部分

| PRD 要求 | 实现位置 | 状态 |
|---|---|---|
| Scenario DSL（fixture / prompt / 文件黑白名单 / 命令必含忌含 / 文本关键字） | [internal/scenario/types.go](../internal/scenario/types.go) | ✅ 字段齐全 |
| Git-based 隔离沙盒（拷 fixture → init/commit → diff → 销毁） | [internal/runner/api.go:152-169](../internal/runner/api.go#L152-L169) + [internal/workspace/workspace.go](../internal/workspace/workspace.go) | ✅ |
| 命令审计（Middleware 拦截） | [internal/runner/tools.go:84](../internal/runner/tools.go#L84) | ✅ 符合 PRD §6.2 |
| 三类硬规则判定（文件 diff / 命令日志 / 输出文本） | [internal/judge/rule.go](../internal/judge/rule.go) | ✅ |
| Markdown + CLI 报告 | [internal/report/json.go](../internal/report/json.go) | ✅ |
| Out of Scope 项（无 GUI / 无 LLM judge / 无 Docker） | 遵守 | ✅ |

---

## 四、代码实现 vs 用户本意：显著偏离

### 偏离 1 — 默认 runner 是 `mock`，而 `mock`/`sandbox` 根本不是 agent（严重）

用户本意是"测 agent 是否守 skill 约束"。但：

- [internal/runner/mock.go](../internal/runner/mock.go) 完全不调模型、不执行命令、不改文件，只是基于 `strings.Contains(skill.Instructions, "react")` 拼一段假输出。
- [internal/runner/sandbox.go:52-61](../internal/runner/sandbox.go#L52-L61) 根据 skill 文本里有没有 `test` / `format` / `lint` 关键字，**自己**往 `ExecutedCmds` 里塞假命令。

这意味着 `mock` / `sandbox` 测的是"**skill 指令文本里有没有某个关键字**"，**不是 agent 的真实行为**。而 `ast.yaml` 默认 `default_runner: mock`，[README.md](../README.md) 还把它列为可用选项。

**后果**：用户 `ast init && ast test` 跑出来一片绿，会误以为 skill 通过了——实际根本没有 agent 在跑。**只有 `--runner=api` 才真正做了用户想要的事**，但它不是默认值。

### 偏离 2 — Skill `tools/` 目录被读了但完全没生效（严重）

PRD §4.1 明确规定 `tools/run_test.json` 是 skill 暴露给 agent 的工具白名单。但：

- [internal/skill/loader.go:68-82](../internal/skill/loader.go#L68-L82) 只把文件名记到 `s.Meta["tools"]`，**没有任何下游消费者**。
- [internal/runner/api.go:199-242](../internal/runner/api.go#L199-L242) 的 `buildToolDefs()` 是**硬编码**的 `read_file / edit_file / run_command / list_files`——不论 skill 声明了什么，都把这四件大杀器交给 agent。

**后果**：skill 想说"我只允许你 run_test、不允许 edit_file"——做不到。skill 的暴露面是固定的，约束完全靠 system prompt 里的自然语言劝诱。PRD §4.1 中"统一 Skill 暴露格式"半残。

### 偏离 3 — `sandbox` runner 把 init 脚本算成 agent 执行的命令（Bug）

[internal/runner/sandbox.go:40](../internal/runner/sandbox.go#L40)：
```go
cmds = append(cmds, sc.Environment.InitScript)
```
init 是**框架自己**为了准备环境跑的，记到 `ExecutedCmds` 里会污染审计结果，也会被 `command_execution.must_not_have` 误判命中。

### 偏离 4 — `matchGlob` 的 `**` 实现不完整

[internal/judge/rule.go:130-151](../internal/judge/rule.go#L130-L151) 只处理 pattern 里恰好一个 `**`，且按"前缀 + 后缀"硬拆。

- `src/**` 这种能匹（前缀 `src/`，后缀空）。
- `src/**/*.go` 这种走 fallback `path.Match`，而 `path.Match` 不支持跨目录——`src/a/b/c.go` 就匹不上。

PRD 示例里用的是 `internal/handler/**` 这种简单形式，demo 不会炸，但稍复杂的 glob 就有**静默漏判**风险（最危险的一种 bug：判定看似通过）。

### 偏离 5 — CLI 命名与 PRD 不一致（小问题）

- PRD §7：`skillcheck run --skill ./skills/go-reviewer --scenarios ./scenarios/go/`
- 实际：`ast test <skill-dir>`（二进制 `ast`，子命令 `test`，scenarios 路径只能从 `ast.yaml` 读，无 `--scenarios` 标志）

语义相近但措辞不同。需对齐其中一边。

### 偏离 6 — 自定义 API endpoint 拼接逻辑可能有坑

[internal/runner/api.go:181-185](../internal/runner/api.go#L181-L185)：
```go
base := strings.TrimSuffix(strings.TrimSuffix(r.cfg.Endpoint, "/messages"), "/")
opts = append(opts, option.WithBaseURL(base+"/"))
```
若代理 endpoint 不以 `/messages` 结尾，被加 `/` 后 SDK 内部还会拼 `/v1/messages`——可能 404。需要在非默认 endpoint 场景下实测。

### 偏离 7 — CLI UX 比 PRD §7 简单很多（次要）

PRD 期望的输出形如：
```
├── [STEP 1] 初始化 Git 隔离沙盒... SUCCESS
├── [STEP 2] 注入用户 Prompt 并驱动 Agent 执行... SUCCESS (耗时 12s)
├── [STEP 3] 收集副作用进行规则审计...
│     ├── 检查文件变动黑白名单... PASSED ...
```
实际只打印 `运行场景: <id>` 和 `[RESULT] PASSED/FAILED`，缺少中间步骤的可视化。

---

## 五、结论

### PRD：没跑偏

`docs/Skill-Check.md` 对用户"测 skill 约束力"的本意刻画准确，技术选型（Git 沙盒、Middleware 命令审计、硬规则 judge）也合理。

### 实现：偏离了一个核心点

MVP 提供了三个 runner，但**默认那个 + sandbox 这个都不是真 agent**——是基于关键字匹配的玩具。用户本意"测成功 = 可上线"，但在默认配置下跑通 `mock`，**毫无上线价值**。这是当前最大的"言行不一致"。

`tools/` 目录被定义但完全没接入运行时，让"Skill Package"作为标准格式的承诺也打了折扣。

---

## 六、修复优先级建议

按 ROI 排序：

| 优先级 | 项目 | 动作 |
|---|---|---|
| **P0** | 偏离 1：默认 runner | 把 `default_runner` 改成 `api`，或 `ast init` 时强制让用户选；在 [README.md](../README.md) 和 `ast init` 输出里**显式警告**：mock/sandbox 只用于自检测试框架，**不能用于验证 skill 是否合格**。激进点可以直接删掉 mock/sandbox |
| **P0** | 偏离 2：tools 白名单未生效 | 解析 `skill/tools/*.json`，让 [buildToolDefs()](../internal/runner/api.go#L199) 基于 skill 声明动态生成工具列表。这是 PRD §4.1 的核心承诺 |
| **P1** | 偏离 3：init 脚本污染审计 | 删除 [sandbox.go:40](../internal/runner/sandbox.go#L40) 那行 append；同样审计 api.go 路径确认无类似污染 |
| **P1** | 偏离 4：glob 不完整 | 换用 `github.com/bmatcuk/doublestar`，或自己实现完整的 `**` 段递归匹配 |
| **P2** | 偏离 5：CLI 命名 | 改代码或改 PRD 其中一边，保持一致 |
| **P2** | 偏离 6：endpoint 拼接 | 用自定义 endpoint 实测一次 |
| **P3** | 偏离 7：CLI UX | 按 PRD §7 补充 STEP 级别的进度打印 |

---

## 七、一句话总结

> **PRD 守得住"测 skill 约束力"的初心，代码守住了骨架但漏了灵魂——默认跑的不是 agent，skill 声明的工具白名单形同虚设。先把 P0 的两条补上，这个工具才真正配得上"过测即可上线"的承诺。**
