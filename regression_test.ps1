<#
.SYNOPSIS
    Alvus 自动回归测试
.DESCRIPTION
    测试单实例模式（回归）和管理模式（新功能），不依赖真实 API 密钥。
    所有测试在临时目录运行，不与用户真实实例冲突。
.NOTES
    设计原则：
    - 每个测试独立端口，互不干扰
    - 自动清理子进程
    - 失败时给出具体错误信息
    - 只测二进制外部行为，不测内部实现
#>

param(
    [string]$AlvusRepo = "D:\Test\Alvus-fork\src",
    [switch]$SkipBuild,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$global:TestPassed = 0
$global:TestFailed = 0
$global:TestSkipped = 0
$global:CleanupJobs = @()

# ── 工具函数 ────────────────────────────────────

function Write-TestResult {
    param([string]$Name, [bool]$Passed, [string]$Detail = "")
    if ($Passed) {
        Write-Host "  ✅ PASS: $Name" -ForegroundColor Green
        $global:TestPassed++
    } else {
        Write-Host "  ❌ FAIL: $Name" -ForegroundColor Red
        if ($Detail) { Write-Host "       $Detail" -ForegroundColor Red }
        $global:TestFailed++
    }
}

function Write-TestHeader {
    param([string]$Name)
    Write-Host "`n━━━ $Name ━━━" -ForegroundColor Cyan
}

function Write-TestSkipped {
    param([string]$Name, [string]$Reason)
    Write-Host "  ⏭️  SKIP: $Name ($Reason)" -ForegroundColor Yellow
    $global:TestSkipped++
}

# 等待 HTTP 端点就绪
function Wait-ForEndpoint {
    param(
        [string]$Url,
        [int]$TimeoutSeconds = 10,
        [int]$ExpectedStatus = 200
    )
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        try {
            $req = [System.Net.HttpWebRequest]::Create($Url)
            $req.Timeout = 1000
            $resp = $req.GetResponse()
            if ($resp.StatusCode -eq $ExpectedStatus) { return $true }
            $resp.Close()
        } catch {
            Start-Sleep -Milliseconds 200
        }
    }
    return $false
}

# 调用 HTTP GET 返回 JSON
function Invoke-AlvusGet {
    param([string]$Url)
    try {
        $req = [System.Net.HttpWebRequest]::Create($Url)
        $req.Timeout = 3000
        $resp = $req.GetResponse()
        $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
        $body = $reader.ReadToEnd()
        $reader.Close()
        $resp.Close()
        return @{ StatusCode = [int]$resp.StatusCode; Body = $body }
    } catch {
        if ($_.Exception.Response) {
            $resp = $_.Exception.Response
            $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
            $body = $reader.ReadToEnd()
            $reader.Close()
            return @{ StatusCode = [int]$resp.StatusCode; Body = $body; Error = $_.Exception.Message }
        }
        return @{ StatusCode = 0; Body = ""; Error = $_.Exception.Message }
    }
}

# 调用 HTTP POST 返回 JSON
function Invoke-AlvusPost {
    param([string]$Url, [string]$JsonBody)
    try {
        $req = [System.Net.HttpWebRequest]::Create($Url)
        $req.Method = "POST"
        $req.ContentType = "application/json"
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($JsonBody)
        $req.ContentLength = $bytes.Length
        $stream = $req.GetRequestStream()
        $stream.Write($bytes, 0, $bytes.Length)
        $stream.Close()
        $resp = $req.GetResponse()
        $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
        $body = $reader.ReadToEnd()
        $reader.Close()
        $resp.Close()
        return @{ StatusCode = [int]$resp.StatusCode; Body = $body }
    } catch {
        if ($_.Exception.Response) {
            $resp = $_.Exception.Response
            $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
            $body = $reader.ReadToEnd()
            $reader.Close()
            return @{ StatusCode = [int]$resp.StatusCode; Body = $body; Error = $_.Exception.Message }
        }
        return @{ StatusCode = 0; Body = ""; Error = $_.Exception.Message }
    }
}

# 启动 alvus 进程
function Start-AlvusProcess {
    param(
        [string]$WorkingDir,
        [string]$BinaryPath,
        [string]$ArgString = "-local",
        [switch]$CaptureOutput
    )
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $BinaryPath
    $psi.Arguments = $ArgString
    $psi.WorkingDirectory = $WorkingDir
    if ($CaptureOutput) {
        $psi.UseShellExecute = $false
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError = $true
    } else {
        $psi.UseShellExecute = $true
    }
    $psi.CreateNoWindow = $true
    $proc = [System.Diagnostics.Process]::Start($psi)
    return $proc
}

# 安全终止进程
function Stop-AlvusProcess {
    param([System.Diagnostics.Process]$Proc)
    if ($Proc -and !$Proc.HasExited) {
        # 杀进程树（manager + 所有子进程），确保不留孤儿
        try { taskkill /F /T /PID $Proc.Id 2>&1 | Out-Null } catch {}
        try { $Proc.WaitForExit(3000) } catch {}
    }
}

# 查找并释放端口
function Get-FreePort {
    $used = $global:AllTestPorts
    if (-not $used) { $global:AllTestPorts = @{}; $used = $global:AllTestPorts }
    for ($p = 15000; $p -lt 16000; $p++) {
        if (-not $used.ContainsKey($p)) {
            $used[$p] = $true
            return $p
        }
    }
    throw "no free port in range 15000-16000"
}

# ── 测试夹具 ────────────────────────────────────

function New-TestFixture {
    param([int]$Port, [hashtable]$EnvVars = @{})

    $tmpDir = Join-Path $env:TEMP "alvus-test-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    # 写入 .env
    $envLines = @(
        "PORT=$Port",
        "TARGET_BASE_URL=https://test.api.example.com/v1",
        "GENAI_BASE_URL=https://genai.test.example.com",
        "API_KEYS=test-key-a,test-key-b,test-key-c",
        "COOLDOWN_SEC=5"
    )
    if ($EnvVars.Count -gt 0) {
        $envLines = $envLines | Where-Object { $_ -notmatch "^PORT=" -and $_ -notmatch "^API_KEYS=" }
        $envLines += "PORT=$Port"
        $envLines += "API_KEYS=$($EnvVars['API_KEYS'])"
    }
    [System.IO.File]::WriteAllLines("$tmpDir\.env", $envLines)

    return $tmpDir
}

function Write-Utf8File {
    param([string]$Path, [string]$Value)
    [System.IO.File]::WriteAllText($Path, $Value, [System.Text.UTF8Encoding]::new($false))
}

function Remove-TestFixture {
    param([string]$Dir)
    if (Test-Path $Dir) {
        Remove-Item -Recurse -Force $Dir -ErrorAction SilentlyContinue
    }
}

# ── 测试: 单实例模式 ─────────────────────────────

function Test-SingleInstanceMode {
    Write-TestHeader "单实例模式（回归测试）"

    $binary = Join-Path $AlvusRepo "alvus.exe"
    if (-not (Test-Path $binary)) {
        Write-TestSkipped "全部" "alvus.exe 不存在，请先 build"
        return
    }

    # ── Test 1: 正常启动 ──
    Write-Host "  ── Test 1: 正常启动 ──" -ForegroundColor Magenta
    $port = Get-FreePort
    $tmpDir = New-TestFixture -Port $port
    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "-local"
        $ready = Wait-ForEndpoint -Url "http://127.0.0.1:$port/health" -TimeoutSeconds 5
        if ($ready) {
            $health = Invoke-AlvusGet "http://127.0.0.1:$port/health"
            Write-TestResult "启动就绪" ($health.StatusCode -eq 200) "健康检查响应: $($health.StatusCode)"
            try { $j = $health.Body | ConvertFrom-Json; Write-TestResult "健康检查返回 JSON" ($j.status -eq "ok" -and $j.keys -eq 3) "status=$($j.status), keys=$($j.keys)" } catch { Write-TestResult "健康检查返回 JSON" $false "JSON 解析失败: $_" }
        } else {
            Write-TestResult "启动就绪" $false "5 秒内未监听 $port"
        }
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 2: 启动后立刻能访问各端点 ──
    Write-Host "  ── Test 2: 端点和 Dashboard ──" -ForegroundColor Magenta
    $port = Get-FreePort
    $tmpDir = New-TestFixture -Port $port
    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "-local"
        $ready = Wait-ForEndpoint "http://127.0.0.1:$port/health" -TimeoutSeconds 5
        if (-not $ready) { Write-TestResult "启动" $false "5秒超时"; return }

        $logs = Invoke-AlvusGet "http://127.0.0.1:$port/logs"
        Write-TestResult "/logs 返回 200" ($logs.StatusCode -eq 200) ""

        $config = Invoke-AlvusGet "http://127.0.0.1:$port/api/config"
        Write-TestResult "/api/config 返回 200" ($config.StatusCode -eq 200) ""

        $dash = Invoke-AlvusGet "http://127.0.0.1:$port/dashboard"
        Write-TestResult "/dashboard 返回 200" ($dash.StatusCode -eq 200) ""
        Write-TestResult "Dashboard 含 HTML" ($dash.Body -match "<!DOCTYPE html>") ""

        # sw.js 应该返回 204
        $sw = Invoke-AlvusGet "http://127.0.0.1:$port/sw.js"
        Write-TestResult "/sw.js 返回 204" ($sw.StatusCode -eq 204) "实际: $($sw.StatusCode)"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 4: 设置项可读可写 ──
    Write-Host "  ── Test 3: 配置读写 ──" -ForegroundColor Magenta
    $port = Get-FreePort
    $tmpDir = New-TestFixture -Port $port
    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "-local"
        if (-not (Wait-ForEndpoint "http://127.0.0.1:$port/health" 5)) { Write-TestResult "启动" $false; return }

        # 修改配置
        $newConfig = @{
            targetBase = "https://new-api.example.com/v1"
            genaiBase  = "https://new-genai.example.com"
            keys       = @("new-key-1", "new-key-2")
        } | ConvertTo-Json

        $post = Invoke-AlvusPost -Url "http://127.0.0.1:$port/api/config" -JsonBody $newConfig

        if ($post.StatusCode -eq 200) {
            Write-TestResult "配置写入成功" $true "POST /api/config → 200"
        } elseif ($post.StatusCode -eq 202) {
            Write-TestResult "配置写入" $true "POST /api/config → 202 (热重载延迟)"
            Start-Sleep 1
        } else {
            Write-TestResult "配置写入成功" $false "POST → $($post.StatusCode): $($post.Body)"
        }

        # 验证写入
        $get = Invoke-AlvusGet "http://127.0.0.1:$port/api/config"
        $j = $get.Body | ConvertFrom-Json
        Write-TestResult "配置 targetBase 已更新" ($j.targetBase -eq "https://new-api.example.com/v1") "期望: new-api.example.com, 实际: $($j.targetBase)"
        Write-TestResult "配置 keys 数正确" ($j.keys.Count -eq 2) "期望 2, 实际 $($j.keys.Count) ($($j.keys -join ','))"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 5: 掩码正确 ──
    Write-Host "  ── Test 4: Key 掩码 ──" -ForegroundColor Magenta
    $port = Get-FreePort
    $tmpDir = New-TestFixture -Port $port -EnvVars @{ API_KEYS = "nvapi-real-key-that-should-be-masked" }
    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "-local"
        if (-not (Wait-ForEndpoint "http://127.0.0.1:$port/health" 5)) { Write-TestResult "启动" $false; return }

        $config = Invoke-AlvusGet "http://127.0.0.1:$port/api/config"
        $j = $config.Body | ConvertFrom-Json
        $firstKey = $j.keys[0]
        $masked = $firstKey -match "\.\.\."
        Write-TestResult "API key 已掩码" $masked "返回: $firstKey"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 6: 日志清空 ──
    Write-Host "  ── Test 5: 日志清空 ──" -ForegroundColor Magenta
    $port = Get-FreePort
    $tmpDir = New-TestFixture -Port $port
    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "-local"
        if (-not (Wait-ForEndpoint "http://127.0.0.1:$port/health" 5)) { Write-TestResult "启动" $false; return }

        $clear = Invoke-AlvusPost -Url "http://127.0.0.1:$port/clear" -JsonBody ""
        Write-TestResult "日志清空 200" ($clear.StatusCode -eq 200) "实际: $($clear.StatusCode)"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }
}

# ── 测试: 管理模式 ─────────────────────────────

function Test-ManageMode {
    Write-TestHeader "管理模式"

    $binary = Join-Path $AlvusRepo "alvus.exe"
    if (-not (Test-Path $binary)) {
        Write-TestSkipped "全部" "alvus.exe 不存在"
        return
    }

    # ── Test 1: 有效 manage.json ──
    Write-Host "  ── Test 1: 正常启动多实例 ──" -ForegroundColor Magenta
    $port1 = Get-FreePort
    $port2 = Get-FreePort
    $tmpDir = Join-Path $env:TEMP "alvus-test-manager-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path "$tmpDir\providers\provider-a" -Force | Out-Null
    New-Item -ItemType Directory -Path "$tmpDir\providers\provider-b" -Force | Out-Null

    Write-Utf8File "$tmpDir\providers\provider-a\.env" @"
PORT=$port1
TARGET_BASE_URL=https://api-a.test.com/v1
API_KEYS=key-a-1,key-a-2
COOLDOWN_SEC=5
"@

    Write-Utf8File "$tmpDir\providers\provider-b\.env" @"
PORT=$port2
TARGET_BASE_URL=https://api-b.test.com/v1
API_KEYS=key-b-1,key-b-2,key-b-3
COOLDOWN_SEC=5
"@

    $manageJson = @"
{
  "providers": [
    { "name": "provider-a", "dir": "providers/provider-a", "port": $port1 },
    { "name": "provider-b", "dir": "providers/provider-b", "port": $port2 }
  ]
}
"@
    Write-Utf8File "$tmpDir\manage.json" $manageJson

    try {
        $proc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "--manage manage.json"
        Start-Sleep -Seconds 2

        $aReady = Wait-ForEndpoint -Url "http://127.0.0.1:$port1/health" -TimeoutSeconds 5
        $bReady = Wait-ForEndpoint -Url "http://127.0.0.1:$port2/health" -TimeoutSeconds 3

        Write-TestResult "Provider A 就绪" $aReady "port $port1"
        Write-TestResult "Provider B 就绪" $bReady "port $port2"

        if ($aReady) {
            $h = Invoke-AlvusGet "http://127.0.0.1:$port1/health"
            Write-TestResult "Provider A 健康检查" ($h.StatusCode -eq 200) "status=$($h.StatusCode)"
        }
        if ($bReady) {
            $h = Invoke-AlvusGet "http://127.0.0.1:$port2/health"
            Write-TestResult "Provider B 健康检查" ($h.StatusCode -eq 200) "status=$($h.StatusCode)"
        }
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 2: 无效 manage.json 路径 ──
    Write-Host "  ── Test 2: 非法配置文件路径 ──" -ForegroundColor Magenta
    $tmpDir = Join-Path $env:TEMP "alvus-test-badconfig-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    try {
        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = $binary
        $psi.Arguments = "--manage nonexistent.json"
        $psi.WorkingDirectory = $tmpDir
        $psi.UseShellExecute = $false
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError = $true
        $psi.CreateNoWindow = $true
        $proc = [System.Diagnostics.Process]::Start($psi)
        $exited = $proc.WaitForExit(5000)
        Write-TestResult "非法路径退出码非零" ($proc.ExitCode -ne 0) "exit code = $($proc.ExitCode)"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }

    # ── Test 3: 无效 JSON ──
    Write-Host "  ── Test 3: 非法 JSON 配置 ──" -ForegroundColor Magenta
    $tmpDir = Join-Path $env:TEMP "alvus-test-badjson-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    try {
        [System.IO.File]::WriteAllText("$tmpDir\bad.json", "this is not json", [System.Text.Encoding]::UTF8)
        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = $binary
        $psi.Arguments = "--manage bad.json"
        $psi.WorkingDirectory = $tmpDir
        $psi.UseShellExecute = $false
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError = $true
        $psi.CreateNoWindow = $true
        $proc = [System.Diagnostics.Process]::Start($psi)
        $exited = $proc.WaitForExit(5000)
        Write-TestResult "非法 JSON 退出码非零" ($proc.ExitCode -ne 0) "exit code = $($proc.ExitCode)"
    } finally {
        Stop-AlvusProcess $proc
        Remove-TestFixture $tmpDir
    }
}

# ── 测试: 子进程管理 ─────────────────────────────

function Test-ProcessManagement {
    Write-TestHeader "子进程生命周期"

    $binary = Join-Path $AlvusRepo "alvus.exe"
    if (-not (Test-Path $binary)) {
        Write-TestSkipped "全部" "alvus.exe 不存在"
        return
    }

    $port = Get-FreePort
    $tmpDir = Join-Path $env:TEMP "alvus-test-procmgmt-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path "$tmpDir\providers\demo" -Force | Out-Null
    Write-Utf8File "$tmpDir\providers\demo\.env" @"
PORT=$port
TARGET_BASE_URL=https://demo.test.com/v1
API_KEYS=demo-key-1
COOLDOWN_SEC=5
"@

    Write-Utf8File "$tmpDir\manage.json" "{ `"providers`": [{ `"name`": `"demo`", `"dir`": `"providers/demo`", `"port`": $port }] }"

    try {
        $mgrProc = Start-AlvusProcess -WorkingDir $tmpDir -BinaryPath $binary -ArgString "--manage manage.json"
        $ready = Wait-ForEndpoint -Url "http://127.0.0.1:$port/health" -TimeoutSeconds 5
        Write-TestResult "子进程启动" $ready "port $port"

        if ($ready) {
            # 找子进程 PID (不是 manager 自己)
            $childPids = @(Get-Process -Name "alvus" -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne $mgrProc.Id } | ForEach-Object { $_.Id })
            Write-TestResult "子进程数量正确" ($childPids.Count -ge 1) "找到 $($childPids.Count) 个子进程"

            # 杀一个子进程，检查自动重启
            if ($childPids.Count -ge 1) {
                $killedPid = $childPids[0]
                try {
                    $childProc = Get-Process -Id $killedPid -ErrorAction Stop
                    $childProc.Kill()
                    $childProc.WaitForExit(2000)
                    Write-Host "      杀死了子进程 PID $killedPid" -ForegroundColor DarkYellow
                } catch {
                    Write-Host "      杀死子进程失败: $_" -ForegroundColor Yellow
                }

                Start-Sleep -Seconds 5
                $restarted = Wait-ForEndpoint -Url "http://127.0.0.1:$port/health" -TimeoutSeconds 8
                Write-TestResult "子进程自动重启" $restarted "杀死后 8 秒内恢复"
            }
        }

        # 停止 manager
        Stop-AlvusProcess $mgrProc
        Start-Sleep -Seconds 1

        $stillRunning = $false
        try {
            $residual = Invoke-AlvusGet "http://127.0.0.1:$port/health" -TimeoutSeconds 1
            if ($residual.StatusCode -gt 0) { $stillRunning = $true }
        } catch {}
        Write-TestResult "Manager 停止后子进程也停止" (-not $stillRunning) ""

    } finally {
        Stop-AlvusProcess $mgrProc
        Remove-TestFixture $tmpDir
    }
}

# ── 主流程 ──────────────────────────────────────

Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║     Alvus 回归测试套件                    ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Cyan

# Step 1: Build
$binary = Join-Path $AlvusRepo "alvus.exe"
if (-not $SkipBuild) {
    Write-Host "`n📦 编译 alvus.exe ..." -ForegroundColor Yellow
    Push-Location $AlvusRepo
    $buildResult = go build -o alvus.exe . 2>&1
    Pop-Location
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ 编译失败: $buildResult" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ 编译成功: $binary" -ForegroundColor Green
}

# Step 2: Cleanup on exit
Register-EngineEvent -SourceIdentifier PowerShell.Exiting -SupportEvent -Action {
    param($event, $sender)
    Write-Host "`n🧹 清理测试进程..." -ForegroundColor Yellow
    # 杀干净的测试进程
    Get-Process -Name "alvus" -ErrorAction SilentlyContinue | Where-Object {
        try {
            $started = $_.StartTime
            $now = [DateTime]::Now
            ($now - $started).TotalMinutes -lt 5
        } catch { $false }
    } | ForEach-Object { try { $_.Kill() } catch {} }
} | Out-Null

# Step 3: Run suites
try {
    Test-SingleInstanceMode
    Test-ManageMode
    Test-ProcessManagement
} catch {
    Write-Host "`n💥 测试异常: $_" -ForegroundColor Red
}

# Step 4: Report
Write-Host "`n═══════════════════════════════════════════" -ForegroundColor Cyan
$total = $global:TestPassed + $global:TestFailed + $global:TestSkipped
$passRate = if ($total -gt 0) { [math]::Round($global:TestPassed / ($global:TestPassed + $global:TestFailed) * 100, 0) } else { 0 }
Write-Host "  总计: $total  |  ✅ PASS: $($global:TestPassed)  |  ❌ FAIL: $($global:TestFailed)  |  ⏭️ SKIP: $($global:TestSkipped)" -ForegroundColor White
Write-Host "  通过率: $passRate%" -ForegroundColor $(if ($global:TestFailed -eq 0) { "Green" } else { "Red" })
Write-Host "═══════════════════════════════════════════" -ForegroundColor Cyan

# Exit code
if ($global:TestFailed -gt 0) { exit 1 }
