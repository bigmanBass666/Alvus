# Tasks

- [x] ~~Task 1: 管理端点鉴权 — 为 ConfigHandler 添加可选令牌检查~~
  - [x] ~~SubTask 1.1: 在 Config 中增加 `AdminToken` 字段和 `ADMIN_TOKEN` 环境变量~~
  - [x] ~~SubTask 1.2: 在 `configHandler` 的 POST 分支添加令牌验证逻辑~~
  - [x] ~~SubTask 1.3: 验证：`go vet ./...` 零警告~~
  - [x] ~~SubTask 1.4: 验证：构建通过 `go build ./...`~~

- [x] ~~Task 2: 共享 HTTP Client — 将 `http.Client` 提升为 `ServerState` 成员~~
  - [x] ~~SubTask 2.1: 为 `ServerState` 添加 `client *http.Client` 字段~~
  - [x] ~~SubTask 2.2: 在 `newServerState` 中初始化带优化 Transport 的 Client~~
  - [x] ~~SubTask 2.3: 移除 `proxyHandler` 中的 per-request Client 创建~~
  - [x] ~~SubTask 2.4: 验证：构建通过，基本请求正常~~

- [x] ~~Task 3: 上游请求传递 Context — 使用 `NewRequestWithContext`~~
  - [x] ~~SubTask 3.1: 将 `http.NewRequest` 改为 `http.NewRequestWithContext(r.Context())`~~
  - [x] ~~SubTask 3.2: 验证：构建通过~~

- [x] ~~Task 4: 日志扫描器缓冲区扩容 — 防止超长日志行静默丢失~~
  - [x] ~~SubTask 4.1: 在 stdout/stderr scanner 中调用 `scanner.Buffer()` 扩容至 1MB~~
  - [x] ~~SubTask 4.2: 添加 `scanner.Err()` 检查~~
  - [x] ~~SubTask 4.3: 验证：构建通过~~

- [x] ~~Task 5: Dashboard HTML 分离 — 使用 `//go:embed` 将 HTML 提取到独立文件~~
  - [x] ~~SubTask 5.1: 创建 `dashboard.html` 文件（提取 dashboardHTML 常量内容）~~
  - [x] ~~SubTask 5.2: 将 `go:embed` 导入添加到 import 块~~
  - [x] ~~SubTask 5.3: 替换 `const dashboardHTML` 为 `var dashboardHTML` 配合 `//go:embed`~~
  - [x] ~~SubTask 5.4: 验证：构建通过，dashboard 页面正常~~

# Task Dependencies

- Task 2 与 Task 3 相互独立，可并行实施
- Task 1 与 Task 5 相互独立，可并行实施
- Task 4 独立，可与其他任务并行