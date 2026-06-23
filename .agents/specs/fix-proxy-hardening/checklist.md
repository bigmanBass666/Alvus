# 代理安全加固 Checkpoints

## 安全加固
- [x] `POST /api/config` 在 `ADMIN_TOKEN` 设置时要求 `X-Admin-Token` 头
- [x] 令牌为空时行为不变（向后兼容）
- [x] GET 请求不受鉴权影响

## 资源优化
- [x] `ServerState` 包含共享 `http.Client` 字段
- [x] Client 配置了 `MaxIdleConns`、`MaxIdleConnsPerHost`、`IdleConnTimeout`
- [x] `proxyHandler` 不再每请求新建 Client
- [x] 上游请求使用 `NewRequestWithContext(r.Context())`
- [x] 客户端断开后，上游请求能被取消（通过 `r.Context()` 传递）

## 日志健壮化
- [x] `bufio.Scanner` 缓冲区扩容至至少 1MB（实际：1MB）
- [x] 添加 `scanner.Err()` 检查并记录日志

## 代码分离
- [x] Dashboard HTML 在独立文件 `dashboard.html` 中（543 行）
- [x] 使用 `//go:embed` 嵌入
- [x] `/dashboard` 返回的内容与重构前一致（原始 HTML 内容未改动）

## 最终验证
- [x] `go build ./...` 成功
- [x] `go vet ./...` 零警告