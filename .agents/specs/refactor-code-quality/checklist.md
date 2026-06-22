# 代码质量重构 Checkpoints

## Bug 修复
- [x] 双抢 sigCh 修复：Ctrl+C 后 5 秒内优雅关闭，无 `server.Shutdown()` 悬空
- [x] 单实例模式：`watchEnvFile` 随 `stop` 信号退出
- [x] 管理模式：`runManager` 随 `stop` 信号关闭所有实例并清理工作目录

## 重复抽取
- [x] `copyHeaders(dst, src http.Header)` 替换了全部 3 处 header 拷贝
- [x] `appendUsageLog()` 替换了全部 2 处 usageLogs 追加
- [x] 重构前后 `go vet ./...` 零警告

## 坏味道清理
- [x] `detectOldConfigFormat()` 已删除
- [x] `Start()` 中无冗余 `os.MkdirAll()` 调用
- [x] `"manage-work"` 已提取为 `const`
- [x] provider name 含 `../` 或 `/` 时返回错误
- [x] `runManager()` 使用 `log.Printf` + `os.Exit(1)` 代替 `log.Fatalf`

## 可读性
- [x] 旧格式检测嵌套深度不超过 4 层
- [x] 代码整体通过自审：没有"看不懂在干嘛"的代码块

## 质量门禁
- [x] `lint.ps1` 脚本存在，运行 `go vet ./...`
- [x] 全部 22 个回归测试通过

## 最终验证
- [x] `go build` 成功
- [x] 回归测试：22/22 ✅
- [x] `go vet ./...`：0 警告
