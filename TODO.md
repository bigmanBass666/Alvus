# Alvus — TODO & 项目建议

> 按优先级和影响范围分组，具体实施时可根据当前聚焦点选择。

---

## ✅ 已完成

### 核心功能
- [x] **配置管理增强** — `internal/config` 包，含 `Validate()`/`Diff()`/`Sanitized()`，启动校验 + 热重载校验 + diff 日志
- [x] **配置管理集成测试** — `integration_test.go`（启动校验 / 热重载 diff / 热重载回滚）
- [x] **日志系统迁移为结构化日志** — 全部 `log.Printf` → `log/slog`，分级 Info/Warn/Error
- [x] **配置变更 diff 日志** — 热重载时记录变更字段（Key 脱敏）
- [x] **关键 Bug 修复** — KeyPool 空池 panic、竞态条件、敏感 Header 泄露、未鉴权端点等 23 项
- [x] **项目结构重构** — `internal/keypool/`、`internal/logstore/`、`internal/utils/` 子包拆分
- [x] **Docker 容器化** — 多阶段构建 `Dockerfile` + `.dockerignore` + `docker-compose.yml`
- [x] **压测与基准测试** — Go Benchmark（KeyPool + Proxy 三场景）+ Vegeta 压测脚本 + mock 上游
- [x] **API Key 名称支持** — `key==name` 格式解析，名称在日志 / API 响应 / Dashboard 全链路展示
- [x] **管理 API 增强** — 5 个新端点（POST disable / PUT cooldown / DELETE {index} / GET stats / POST reload）
- [x] **可观测性：Prometheus Metrics** — 4 指标 + `/metrics` 端点 + 埋点 + 自定义 Registry 隔离
- [x] **Metrics 验收测试** — 6 集成验收测试
- [x] **CLAUDE.md 测试策略对齐** — 明确 Testing Trophy 模型
- [x] **Error Handling 统一** — 4 错误码 + 统一 JSON 错误响应
- [x] **两层熔断器** — KeyCircuitBreaker（三态 + 指数退避）+ UpstreamCircuitBreaker（三态）
- [x] **main.go 拆包** — 977 行 → 98 行，5 文件拆分
- [x] **启动配置友好校验** — 中文错误消息 + 标准退出码
- [x] **README 重写** — 450+ 行完整文档
- [x] **Key 持久化存储** — `keys.json` JSON 文件存储，管理 API 写操作自动同步
- [x] **优雅关闭（Graceful Shutdown）** — `http.Server.Shutdown()` + `sync.WaitGroup` + 30s 超时
- [x] **上游健康检查** — 主动健康检查 goroutine + 3 配置字段 + 3 Prometheus 指标
- [x] **Docker Compose 完整部署** — 三服务（Alvus + Prometheus + Grafana），持久化数据卷，预置监控面板
- [x] **Key 加密存储** — AES-256-GCM 加密模块，`SaveFullStore`/`LoadFullStore` 自动加密/解密
- [x] **日志系统增强** — `LOG_LEVEL` 启动生效 + `POST /api/log-level` 动态切换 + Debug 级别请求/响应体日志（敏感数据清洗）+ 结构化字段命名标准化
- [x] **技术债修复** — KeyPool RWMutex、LoadConfig/ReloadConfig 去重、Diff 注册表模式、ProxyEngine 提取
- [x] **错误分类细化** — ErrorCategory（CatRetryable/CatNonRetryable/CatClientAbort）+ categorizeError 函数，NonRetryable 4xx 不消耗重试次数、ClientAbort 不惩罚 Key 健康度

### 191 测试覆盖

| 文件 | 测试数 | 类型 |
|------|--------|------|
| `internal/config/config_test.go` | 30 | 单元测试 |
| `internal/keypool/keypool_test.go` | 12 | 单元测试 |
| `internal/keypool/store_test.go` | 14 | 单元测试 |
| `internal/keypool/crypto_test.go` | 9 | 单元测试 |
| `internal/logstore/logstore_test.go` | 5 | 单元测试 |
| `internal/circuitbreaker/key_test.go` | 10 | 单元测试 |
| `internal/circuitbreaker/upstream_test.go` | 9 | 单元测试 |
| `internal/server/logging_test.go` | 15 | 单元测试 |
| `internal/server/error_classification_test.go` | 6 | 单元测试 |
| `handlers_test.go` | 20 | Handler 测试 |
| `logstore_test.go` | 4 | Handler 测试 |
| `proxy_test.go` | 34 | **集成验收测试** |
| `integration_test.go` | 4 | **集成测试** |
| `metrics_verification_test.go` | 6 | **集成验收测试** |
| `graceful_shutdown_test.go` | 3 | **集成验收测试** |
| `healthcheck_test.go` | 5 | **集成验收测试** |
| `docker_compose_test.go` | 5 | **集成验收测试** |
| **总计** | **191** | |

---

## 🎯 焦点

*当前未选定焦点事项。*

*ccswitch 源码分析报告 → `.agents/documents/ccswitch-analysis.md`*

---

## P1 — 快速见效（一天级）

### CD / Release Pipeline
- GoReleaser 自动发布多平台二进制到 GitHub Releases
- Docker 镜像自动构建推送到 ghcr.io
- Semantic Release + 自动 changelog
- 配 `git tag v0.1.0` 触发

**前提**：Dockerfile + CI 已就位，只需配 GoReleaser 和 Release workflow。

---

## P2 — 值得做（优化打磨，数天级）

### 性能优化
**有压测数据支撑**：50 QPS p99 15.8ms ✅ / 200 QPS ⚠️ 开始饱和 / 500 QPS ❌ 32% 成功

- **Key 选择策略可配置** — round-robin → 支持 least-loaded / weighted / priority 策略
- **HTTP 连接池调优** — 当前 MaxIdleConns=100, MaxIdleConnsPerHost=10，需压测验证瓶颈
- **请求 body 零拷贝转发** — `io.CopyN` / `splice` 减少内存拷贝（proxy 流式转发场景）
- **Vegeta 基准重测** — 优化后重跑压测，验证 QPS 提升

### 安全性增强（续）
- 管理 API 返回 Key 统一脱敏（部分已实现，待全链路对齐）
- 可选从外部密钥管理服务读取 Key（Vault / AWS Secrets Manager）

### 优雅降级
- 无可用 Key 时重试队列（暂存请求等待 Key 恢复）
- 无可用上游时降级（友好错误提示）
- 半开状态（允许少量探测请求判断恢复）

---

## P3 — 锦上添花（长期愿景）

### Dashboard 增强
- 请求日志详情页
- Key 使用量统计图表
- 实时日志流（WebSocket）
- 配置管理界面

### CLI 管理工具
- `alvusctl` — 独立 CLI，通过管理 API 操作运行中代理
  - `alvusctl keys list / add / remove`
  - `alvusctl stats / reload`

### 请求/响应预处理
- Header 过滤/注入（非透传场景）
- Stream 模式优化（SSE 流式响应处理）
- 响应格式转换

---

## ⚠️ 已知约束

- **ccswitch 领域不碰** — 格式化/整流/转发、provider 路由、请求修改、响应变换、DISABLE_THINKING 等 ccswitch 已成熟的功能不重复造轮。Alvus 定位为 **"单 provider 内的 API Key 轮转"**，与 ccswitch 互补。详见 `.agents/documents/ccswitch-analysis.md`。
- **WSL2 9p 文件系统不支持 inotify** — 容器内热重载不会触发（不影响裸跑）
- **高并发性能瓶颈** — 100+ QPS 开始饱和，属于优化范围不影响功能正确性
- **Alvus 非必要不复杂化** — 当前熔断器足以应付 Key 轮转场景（Key 切换成本极低），无须对标 ccswitch 的 HalfOpen/错误率等精细熔断机制

## 附：历史验证摘要

### 压测基线（参考）
| 场景 | 结果 |
|------|------|
| 50 QPS 冒烟测试 | 100% 成功，p99 15.8ms |
| 500 QPS 全量压测 | ⚠️ 32% 成功，p99 34s（i5 笔记本饱和） |
| 200 QPS 中等负载 | ⚠️ 1.6% 成功（大量超时，需性能调优） |

### Docker 验证
- `docker compose config` ✅ 语法通过
- CI Docker build ✅ 已在 go.yml 中配置
- WSL2 网络限制（Docker Hub 不可达），但 CI 可正常构建

### 熔断器验证
- KeyCircuitBreaker: 429 触发指数退避，401/403 永久禁用 ✅
- UpstreamCircuitBreaker: 5xx 触发熔断，304 恢复 ✅
- 上游错误不惩罚 Key（设计正确） ✅

---

关于这个cli我是这样想的, 我是想弄成全部都由一个alvus来管理, 就好像ccswitch cli也是一个cli管全部. 一个alvus命令可以增删改查provider, key, 然后也可以用alvus启动特定的provider或者多个provider, 总之就是我们把alvus封装成一个cli就是了, 你觉得这种设计如何呢? 说说你的看法
