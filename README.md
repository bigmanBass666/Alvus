# ⚡ Alvus — Multi-Instance Fork

> 上游原版: [OmitNomis/Alvus](https://github.com/OmitNomis/Alvus)
>
> 本 fork 新增: **管理模式**（`--manage`），一个命令启动/管理多个 API 供应商代理。

---

## 目录

- [这是什么](#这是什么)
- [快速上手](#快速上手)
- [单实例模式（跟原版一样）](#单实例模式跟原版一样)
- [管理模式（本 fork 新增）](#管理模式本-fork-新增)
- [如何新增一个供应商](#如何新增一个供应商)
- [常见场景](#常见场景)
- [目录结构说明](#目录结构说明)
- [回归测试](#回归测试)

---

## 这是什么

**Alvus** 是一个零依赖的 Go 反向代理。你给它一堆 API key，它在中间做轮换——遇到 429/502/503 自动换下一个 key，你的 AI 工具永远看不见限流错误。

**本 fork** 在保留所有原功能的基础上，加了**管理模式**：一个 `alvus.exe` 同时管理多个供应商实例（比如 NVIDIA、OpenAI、DeepSeek 各跑一个，互不干扰）。

---

## 快速上手

```bash
# 编译（需要 Go 1.21+）
cd src && go build -o alvus.exe .

# 双击启动单实例（跟原来一样）
./alvus.exe -local

# 管理模式启动多供应商
./alvus.exe --manage manage.json
```

---

## 单实例模式（跟原版一样）

用法跟原版完全一致，没有任何行为变化。

### 配置 `.env`

```env
PORT=4000
TARGET_BASE_URL=https://integrate.api.nvidia.com/v1
GENAI_BASE_URL=https://ai.api.nvidia.com
API_KEYS=key1,key2,key3
COOLDOWN_SEC=60
```

### 启动

```bash
./alvus.exe -local       # 只监听本机
./alvus.exe -network-only # 监听局域网
```

### 检查

浏览器打开 `http://localhost:4000/dashboard` 或 `http://localhost:4000/health`。

---

## 管理模式（本 fork 新增）

### 什么是管理模式

```
你执行:
  ./alvus.exe --manage manage.json

发生了什么:
  ┌─ alvus.exe (管理器) ─────────────────────┐
  │                                           │
  │  ├─ 启动子进程: NVIDIA (端口 4000)        │
  │  │   └─ 读取 proxies/nvidia/.env          │
  │  │   └─ key 轮换 → 上游 NVIDIA NIM       │
  │  │                                         │
  │  ├─ 启动子进程: OpenAI (端口 4001)        │
  │  │   └─ 读取 proxies/openai/.env          │
  │  │   └─ key 轮换 → 上游 OpenAI API        │
  │  │                                         │
  │  └─ 每个子进程独立互不干扰                 │
  │                                             │
  │  挂了自动重启 ✓  Ctrl+C 全关 ✓            │
  └─────────────────────────────────────────────┘
```

每个子进程都是一个独立的 `alvus.exe`，有自己的 `.env`、自己的端口、自己的 key 池。一个崩了不影响其他。

### 准备 manage.json

在 `alvus.exe` 同目录下创建 `manage.json`：

```json
{
  "providers": [
    {
      "name": "nvidia",
      "dir": "proxies/nvidia",
      "port": 4000
    },
    {
      "name": "openai",
      "dir": "proxies/openai",
      "port": 4001,
      "disabled": true
    }
  ]
}
```

字段说明：

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | ✅ | 供应商名称，用于日志标识 |
| `dir` | ✅ | 子进程的工作目录（包含 `.env`） |
| `port` | ✅ | 该实例监听的端口 |
| `disabled` | ❌ | `true` 时跳过此实例 |

> `dir` 是相对于 `manage.json` 所在目录的路径。

### 启动管理模式

```bash
./alvus.exe --manage manage.json
```

你会看到：

```
✅ [nvidia] started (PID 12345, port 4000, dir: .../proxies/nvidia)
✅ [openai] started (PID 12346, port 4001, dir: .../proxies/openai)
🚀 Manager: 2/2 instances started
```

### 停止

按 `Ctrl+C`，管理器会：
1. 发送停止信号给所有子进程
2. 等待子进程退出
3. 自己退出

### 自动重启

如果某个子进程意外崩溃了，管理器每 3 秒检查一次，自动拉起来。

---

## 如何新增一个供应商

### 步骤

举个例子：你要加一个 DeepSeek 供应商。

**1. 创建目录和 `.env`**

```
proxies/
├── nvidia/
│   └── .env
├── openai/
│   └── .env
└── deepseek/          ← 新建
    └── .env           ← 新建
```

**2. 编写 `.env`**

```env
PORT=4002
TARGET_BASE_URL=https://api.deepseek.com/v1
API_KEYS=sk-deepseek-key-1,sk-deepseek-key-2
COOLDOWN_SEC=60
```

> ⚠️ 每个供应商的 `PORT` 必须不同，不能冲突。

**3. 在 manage.json 加一行**

```json
{
  "providers": [
    { "name": "nvidia",  "dir": "proxies/nvidia",  "port": 4000 },
    { "name": "openai",  "dir": "proxies/openai",  "port": 4001 },
    { "name": "deepseek", "dir": "proxies/deepseek", "port": 4002 }
  ]
}
```

**4. 重启管理器**

按 `Ctrl+C` 关掉，重新 `./alvus.exe --manage manage.json`。

搞定。

---

## 常见场景

### 我只想跑 NVIDIA

```json
{
  "providers": [
    { "name": "nvidia", "dir": "proxies/nvidia", "port": 4000 }
  ]
}
```

或者直接用单实例模式，不用 manage.json。

### 我想暂时关掉某个供应商，不删配置

加 `"disabled": true`：

```json
{ "name": "openai", "dir": "proxies/openai", "port": 4001, "disabled": true }
```

下次启动时会跳过它。

### 我不想每次手动重启管理器

用 `start-all.ps1`（已配好）：

```powershell
.\start-all.ps1
```

它会为每个供应商弹一个独立窗口。

---

## 目录结构说明

```
alvus-fork/
├── src/
│   ├── main.go               # 单实例逻辑（跟原版一样）
│   ├── manage.go              # 管理模式（本 fork 新增）
│   ├── go.mod                 # module 声明，零依赖
│   ├── manage.json            # 管理模式配置文件
│   └── regression_test.ps1    # 回归测试脚本（22 个用例）
│
├── proxies/                   # 你的供应商目录（按需创建）
│   ├── nvidia/
│   │   └── .env
│   └── openai/
│       └── .env
│
├── start-nvidia.ps1           # 单独启动 NVIDIA
├── start-openai.ps1           # 单独启动 OpenAI
├── start-all.ps1              # 一键启动所有
└── start-provider-template.ps1 # 新供应商启动脚本模板
```

---

## 回归测试

不需要 API key，在本地就能跑：

```powershell
.\regression_test.ps1
```

测试内容：
- 单实例模式：启动、健康检查、配置读写、Dashboard、Key 掩码、日志清空
- 管理模式：多实例启动、非法配置处理
- 进程管理：自动重启、停止传播

---

> 有问题开 Issue，或者直接找我。