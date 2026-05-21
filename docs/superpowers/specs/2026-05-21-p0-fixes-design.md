# P0 修复设计：默认 runner 切换 + tools 白名单接入

> 关联文档：[../../Implementation-Review.md](../../Implementation-Review.md)
> 状态：Design — 待实现
> 范围：仅 P0 两项偏离（默认 runner、tools 白名单）。P1/P2/P3 不在本次范围。

---

## 1. 背景与目标

[Implementation-Review.md](../../Implementation-Review.md) 列出了实现相对 PRD 的 7 项偏离。其中 P0 两项直接危及"过测即可上线"这一核心承诺：

- **P0-1**：默认 `runner: mock` 不是真 agent，只做关键字匹配。用户 `ast init && ast test` 一片绿，毫无验证价值。
- **P0-2**：skill 的 `tools/*.json` 被读了但没被消费，所有 skill 都强制拿到 `read_file / edit_file / run_command / list_files` 四件套——skill 的工具白名单形同虚设。

本设计修复这两点，让"过测 = skill 守住了它声明的工具暴露面 + 真实 agent 行为符合约束"。

## 2. 非目标

- 不删除 mock/sandbox runner，仅降权 + 警告（用户已决策）。
- 不修复 P1（glob、init 命令污染）、P2（CLI 命名、endpoint 拼接）、P3（CLI UX）。
- 不引入 LLM-as-a-judge，不改判定层。
- 不改 `Runner` 接口签名。

---

## 3. P0-1：默认 runner 改为 api + 警告

### 3.1 修改清单

| 文件 | 改动 |
|---|---|
| [internal/config/config.go](../../../internal/config/config.go) | `Default()` 中 `DefaultRunner: "mock"` → `"api"`；`Load()` 兜底分支同步改 |
| [cmd/ast/main.go](../../../cmd/ast/main.go) `cmdTest` | runner 解析后若为 `mock`/`sandbox`，stderr 打印高亮警告块 |
| [cmd/ast/main.go](../../../cmd/ast/main.go) `cmdInit` | 生成的 `ast.yaml` 在 `default_runner` 上加注释说明三种值的语义 |
| [README.md](../../../README.md) | Runners 表格加粗 mock/sandbox 的局限；把 api runner 用法块上移作为推荐 |

### 3.2 警告文案（最终落地以代码为准）

```
[WARN] ────────────────────────────────────────────────────────────
[WARN] Runner "mock" does NOT invoke a real agent.
[WARN] Pass/fail results CANNOT be used to validate skill compliance.
[WARN] Use --runner=api (or set default_runner: api in ast.yaml) for
[WARN] real behavior testing before treating any result as shippable.
[WARN] ────────────────────────────────────────────────────────────
```

### 3.3 风险

- 现有用户升级后跑 `ast test` 若未配置 API key，会从"假绿"变成"真红"（API runner 报错）。这是**期望行为**——隐式 false positive 比显式失败危险得多。README 升级提示需覆盖。

---

## 4. P0-2：tools 白名单接入

### 4.1 数据结构

新增 [internal/skill/types.go](../../../internal/skill/types.go) 中：

```go
type ToolDef struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    InputSchema map[string]any `json:"input_schema"`
}
```

修改 `Skill` 结构体：
- 移除 `Tools []byte`（当前未使用，无消费者）
- 新增 `ToolDefs []ToolDef`

### 4.2 Skill 加载器

[internal/skill/loader.go](../../../internal/skill/loader.go) 的 `LoadFromDir`：

- 遍历 `tools/` 目录下所有 `*.json` 文件
- 每个文件按 `ToolDef` 反序列化
- 校验：`Name` 非空、`InputSchema` 非空；任一失败则返回错误（fail-fast，避免静默吞掉错配的 skill）
- 反序列化结果填入 `Skill.ToolDefs`
- 保留 `s.Meta["tools"]` 行为用于报告展示（值改为名字列表）

### 4.3 内置工具注册表

新增 [internal/runner/tool_registry.go](../../../internal/runner/tool_registry.go)：

```go
// builtinTools 是 ast 提供的开箱即用工具集。
// skill 可以在 tools/ 目录中通过名字引用它们，或定义完全自定义的工具。
var builtinTools = map[string]anthropic.ToolParam{
    "read_file":   { ... },
    "edit_file":   { ... },
    "run_command": { ... },
    "list_files":  { ... },
}
```

从 [internal/runner/api.go](../../../internal/runner/api.go) 当前硬编码的 `buildToolDefs()` 中迁移定义，函数本体改为按名字查表。

### 4.4 工具组装逻辑

[internal/runner/api.go](../../../internal/runner/api.go) 的 `buildToolDefs` 改签名：

```go
func buildToolDefs(sk skill.Skill) ([]anthropic.ToolUnionParam, error)
```

行为分支：

| 情况 | 行为 |
|---|---|
| `sk.ToolDefs` 为空（skill 无 `tools/` 目录） | 返回全部 4 个内置工具（向后兼容） |
| `sk.ToolDefs` 非空，且 `def.InputSchema` 为空 | 视为"引用内置工具"，从 `builtinTools` 按 `def.Name` 取出；未找到则返回 error |
| `sk.ToolDefs` 非空，且 `def.InputSchema` 非空 | 视为"自定义工具"，用 `def` 构造 `ToolParam`（注意：runner 没有该工具的执行器，会路由到 `ToolExecutor.Execute` 的 default 分支返回 error）。**MVP 阶段先这么处理，后续可扩展插件式 executor。** |

注意第三种情况会导致自定义工具被模型调用时返回错误。**这是有意为之**——skill 作者声明了一个 ast 不认识的工具，行为应该可见地失败，而不是默默被吞。

### 4.5 工具执行器

[internal/runner/tools.go](../../../internal/runner/tools.go) 的 `ToolExecutor.Execute` 不变。default 分支已经返回 `unknown tool` 错误，对应 4.4 第三种情况。

### 4.6 示例 skill

`cmd/ast/main.go` 的 `cmdInit` 在创建 `./scenarios/example.yaml` 的同时，**额外**创建 `./skills/example-skill/`：

```
skills/example-skill/
├── skill.yaml          # id, name, description
├── instructions.md     # 简单的 system prompt 示例
└── tools/
    ├── read_file.json  # 仅声明名字，引用内置
    └── edit_file.json
```

用户可直接 `ast test ./skills/example-skill --runner=api` 跑通闭环。

### 4.7 mock / sandbox runner

不消费 `ToolDefs`。它们本就不是真 agent，强行让它们模拟工具白名单只会增加伪绿色面积。保持现状 + P0-1 的警告即可。

---

## 5. 测试策略

无既有单元测试。本次新增覆盖以下场景的最小测试（位置：`internal/skill/loader_test.go`、`internal/runner/api_test.go`）：

| 场景 | 断言 |
|---|---|
| skill 无 `tools/` 目录 | `Skill.ToolDefs` 长度为 0，`buildToolDefs` 返回 4 个内置工具 |
| skill 含 `tools/read_file.json`（无 schema） | `buildToolDefs` 仅返回 1 个工具，且为内置 `read_file` |
| skill 含 `tools/custom.json`（带完整 schema） | `buildToolDefs` 返回该自定义工具 |
| skill 含 `tools/typo.json`（引用了不存在的内置名） | `buildToolDefs` 返回 error |
| `tools/*.json` 缺字段（无 `name`） | `LoadFromDir` 返回 error |

`buildToolDefs` 的 mock/sandbox 路径无需测试——它们不调用该函数。

API runner 的端到端测试不在本次范围（需 API key）。

---

## 6. 实施顺序建议

1. P0-2 的数据结构 + loader 改动（无副作用，可独立）
2. P0-2 的注册表 + buildToolDefs 改动 + 单元测试
3. P0-2 的示例 skill
4. P0-1 的默认值切换 + 警告 + 文档（最后做，避免在 tools 调通前默认 runner 就失败）

---

## 7. 验收标准

- 在没有 API key 的环境下跑 `ast init && ast test ./skills/example-skill`，应**显式失败**并提示需要 `ANTHROPIC_API_KEY`，而不是返回伪绿色结果。
- 强制 `--runner=mock` 时，输出顶部出现 4.2 节定义的警告块。
- 准备一个 `tools/` 只声明 `read_file` 的 skill，让 api runner 跑起来。模型若调用 `edit_file`，应该收到 `tool not available` 类错误（由 SDK 层在工具未声明时拒绝，或我方 executor 返回 unknown tool）。
- 单元测试全绿。
