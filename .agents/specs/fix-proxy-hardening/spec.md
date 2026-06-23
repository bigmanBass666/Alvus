# 代理安全加固与资源优化 Spec

## Why

代码审查发现 Alvus 存在安全漏洞（管理端点无鉴权）、资源浪费（每请求建连接池、Context 不传递）、健壮性缺陷（日志长行静默丢失）。这些问题在高并发生产场景下会导致安全隐患和性能退化。

## What Changes

- **安全加固**：为 `/api/config` 管理端点添加鉴权
- **资源复用**：共享 `http.Client` 实例，传递 Context 取消上游请求
- **日志健壮化**：`bufio.Scanner` 支持超长日志行
- **代码分离**：将 Dashboard HTML 使用 `//go:embed` 分离到独立文件

## Impact

- 改动范围：`main.go`、`manage.go`、新增 `dashboard.go`（或 `dashboard.html`）
- 新增约束：管理 API 操作需要 `X-Admin-Token` 头
- 向后兼容：默认 `AdminToken` 为空时不鉴权（保持原有行为）

## ADDED Requirements

### Requirement: 管理端点鉴权

系统 SHALL 为 `POST /api/config` 提供可选的令牌鉴权。

#### Scenario: 未提供令牌的修改请求被拒绝
- **WHEN** 客户端向 `POST /api/config` 发送请求，不带 `X-Admin-Token` 头
- **AND** 服务器配置了 `ADMIN_TOKEN` 环境变量
- **THEN** 返回 HTTP 401 Unauthorized

#### Scenario: 提供正确令牌的修改请求被接受
- **WHEN** 客户端向 `POST /api/config` 发送请求，带 `X-Admin-Token: <正确令牌>`
- **THEN** 正常处理配置更新

#### Scenario: 令牌为空时行为不变
- **WHEN** `ADMIN_TOKEN` 环境变量未设置或为空
- **THEN** `/api/config` 端点的 GET/POST 行为不变

### Requirement: Dashboard 静态资源分离

系统 SHALL 将内联的 dashboard HTML 字符串从 `main.go` 分离到独立静态文件。

#### Scenario: Dashboard 页面正常渲染
- **WHEN** 访问 `/dashboard`
- **THEN** 返回与原来完全相同的 HTML 内容

## MODIFIED Requirements

### Requirement: 上游请求传递 Context

`proxyHandler` 中的上游请求 SHALL 使用 `http.NewRequestWithContext` 传递客户端 Context，使客户端断开时能取消上游请求。见 `main.go:501`。

### Requirement: 共享 HTTP Client

`http.Client` SHALL 从每请求创建改为 `ServerState` 成员共享，复用底层连接池。见 `main.go:454`。

### Requirement: 日志扫描器缓冲区扩容

`manage.go` 中的 `bufio.Scanner` SHALL 扩容至 1MB，并添加 `scanner.Err()` 检查，防止超长日志行静默丢失。见 `manage.go:104-116`。

## REMOVED Requirements

（无移除项）