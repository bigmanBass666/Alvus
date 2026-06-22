# 代码质量重构 Spec

## Why

之前的功能开发流程有缺陷：写完功能跑通测试就算"做完"，没有自我审查、没有跑静态分析、重复代码直接复制不提炼。结果产生了一批 bug（双抢信号导致进程挂住）、坏味道（死代码、深层嵌套、硬编码）、和维护痛点（165 行函数、500 行内联 HTML）。

## What Changes

- **修复 Bug**：双抢 sigCh 导致 graceful shutdown 失效
- **消除重复**：把 header 拷贝、usageLogs 追加抽成共享函数
- **清理坏味道**：删死代码、去冗余调用、常量代替硬编码、修复目录穿越
- **提高可读性**：拆大函数、降嵌套深度
- **建立质量门禁**：改完代码必须自审 diff + 跑 `go vet`

## Impact
- 改动范围：`manage.go`、`main.go`
- 无功能变更，纯重构 + bug 修复
- 回归测试（22 个用例）必须全部通过

## ADDED Requirements

### Requirement: 代码自审

开发者提交前 SHALL：
- 重读自己的 diff，找出死代码、重复结构
- 确认每行改动都能追溯到需求
- 不复制相同结构三次以上

### Requirement: 静态分析门禁

开发者 SHALL 在提交前运行 `go vet ./...`，零警告才能提交。

## MODIFIED Requirements

### Requirement: 单实例 graceful shutdown（修复 Bug）

原代码中两个 goroutine 同时监听 `sigCh`，第二个始终收不到信号，导致 `server.Shutdown()` 不被执行。

修改后 SHALL：
- 使用 `stop` channel 作为单一信号消费通道
- signal handler 关闭 `stop`
- `server.Shutdown()` 由 `stop` 的关闭触发
- 按 Ctrl+C 后，5 秒内完成 in-flight 请求的优雅关闭

### Requirement: 重复逻辑抽取

原代码中以下重复模式 SHALL 被提取为共享函数：
- Header 拷贝（出现 3 次）→ `copyHeaders(dst, src http.Header)`
- UsageLogs 追加（出现 2 次）→ `appendLog(entry LogEntry)`

### Requirement: 死代码清理

`detectOldConfigFormat()` 函数 SHALL 被删除。其逻辑已被内联到 `LoadManagerConfig()` 中。

### Requirement: 冗余调用移除

`ManagedInstance.Start()` 中的 `os.MkdirAll()` SHALL 被移除，因为 `writeEnvFile()` 已确保目录存在。

### Requirement: 常量提取

- `"manage-work"` SHALL 被提取为 `const workDirName = "manage-work"`

### Requirement: 目录穿越防护

`NewManager()` 中 SHALL 验证 provider name 的有效性，防止包含 `../` 或路径分隔符。

### Requirement: `log.Fatalf` 替换

`runManager()` 中的 `log.Fatalf()` SHALL 被替换为 `log.Printf()` + `return`，确保 `defer` 和清理代码得以执行。

### Requirement: 深层嵌套降低

- 旧格式检测 6 层类型断言 SHALL 被提取为单独函数 `detectOldFormat()` 并尽早 return
- 嵌套深度 SHALL 不超过 4 层

## REMOVED Requirements

（无移除项）
