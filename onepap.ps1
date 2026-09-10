# ONEPAP.24 unified local manager for Windows / PowerShell 7.
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet("start", "stop", "restart", "status", "open", "build", "test", "check", "clean", "doctor", "logs", "help")]
    [string]$Action = "help",

    [int]$Port = -1,
    [string]$HostAddress = "",
    [string]$Config = "",
    [switch]$StrictPort,
    [switch]$Foreground,
    [switch]$NoBrowser,
    [switch]$ServerOnly,
    [switch]$Deep
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = $PSScriptRoot
$AppDir = Join-Path $Root "configurator"
$DesktopDir = Join-Path $Root "desktop\ONEPAP.24_Modbus_Configurator"
$RuntimeDir = Join-Path $Root ".runtime"
$RuntimeFile = Join-Path $RuntimeDir "server.json"
$StdoutLog = Join-Path $RuntimeDir "server.stdout.log"
$StderrLog = Join-Path $RuntimeDir "server.stderr.log"
$ArtifactsDir = Join-Path $Root "artifacts"
$BackendBinary = Join-Path $ArtifactsDir "modbus-backend.exe"
$Executable = Join-Path $ArtifactsDir "onepap-modbus-configurator.exe"
$DefaultConfig = Join-Path $AppDir "configurator.toml"

function Write-Section([string]$Title) {
    Write-Host "`n=== $Title ===" -ForegroundColor Cyan
}

function Assert-Command([string]$Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "Не найдена команда '$Name'. Установите её и повторите запуск."
    }
    return $command
}

function Resolve-ConfigPath {
    if ([string]::IsNullOrWhiteSpace($Config)) {
        return $DefaultConfig
    }

    $candidate = $Config
    if (-not [System.IO.Path]::IsPathRooted($candidate)) {
        $candidate = Join-Path (Get-Location) $candidate
    }
    if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
        throw "Файл конфигурации не найден: $candidate"
    }
    return (Resolve-Path -LiteralPath $candidate).Path
}

function Read-RuntimeInfo {
    if (-not (Test-Path -LiteralPath $RuntimeFile -PathType Leaf)) {
        return $null
    }
    try {
        return Get-Content -LiteralPath $RuntimeFile -Raw | ConvertFrom-Json
    }
    catch {
        Write-Warning "Runtime-файл повреждён и будет удалён: $RuntimeFile"
        Remove-Item -LiteralPath $RuntimeFile -Force -ErrorAction SilentlyContinue
        return $null
    }
}

function Get-ManagedProcess {
    $info = Read-RuntimeInfo
    if (-not $info -or -not $info.pid) {
        return $null
    }

    $process = Get-Process -Id ([int]$info.pid) -ErrorAction SilentlyContinue
    if (-not $process) {
        Remove-Item -LiteralPath $RuntimeFile -Force -ErrorAction SilentlyContinue
        return $null
    }

    return [PSCustomObject]@{
        Info = $info
        Process = $process
    }
}

function Invoke-InAppDirectory([scriptblock]$Command) {
    Push-Location $AppDir
    try {
        & $Command
        if ($LASTEXITCODE -ne 0) {
            throw "Команда завершилась с кодом $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}

function Test-BinaryNeedsBuild {
    if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
        return $true
    }

    $binaryTime = (Get-Item -LiteralPath $Executable).LastWriteTimeUtc
    $sourcePatterns = @("*.go", "*.html", "*.css", "*.js", "go.mod", "go.sum", "configurator.toml", "package.json", "vite.config.js")
    $newerSource = Get-ChildItem -LiteralPath @($AppDir, $DesktopDir) -Recurse -File -Include $sourcePatterns |
        Where-Object { $_.LastWriteTimeUtc -gt $binaryTime } |
        Select-Object -First 1
    return $null -ne $newerSource
}

function Build-OnePap {
    Write-Section "Сборка единого десктопного приложения (Go + Wails v3)"
    [void](Assert-Command "go")
    New-Item -ItemType Directory -Path $ArtifactsDir -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $DesktopDir "backend") -Force | Out-Null

    $targetBackend = Join-Path $DesktopDir "backend\modbus-backend.exe"

    Write-Host "[1/2] Компиляция встроенного Go-бэкенда..." -ForegroundColor Cyan
    Invoke-InAppDirectory {
        go mod download
        if ($LASTEXITCODE -ne 0) { throw "Не удалось загрузить Go-модули" }
        go build -trimpath -o $targetBackend .
    }

    Write-Host "[2/2] Компиляция Wails v3 Desktop оболочки..." -ForegroundColor Cyan
    Push-Location $DesktopDir
    try {
        wails3 build
        if ($LASTEXITCODE -ne 0) { throw "Ошибка сборки Wails v3 desktop приложения" }
        Copy-Item -LiteralPath (Join-Path $DesktopDir "bin\onepap-24-modbus-configurator.exe") -Destination $Executable -Force
    }
    finally { Pop-Location }

    # Удаляем старые/промежуточные бинарники из artifacts, оставляя строго один единый бинарник
    Get-ChildItem -LiteralPath $ArtifactsDir -Filter "*.exe" |
        Where-Object { $_.FullName -ne $Executable } |
        Remove-Item -Force -ErrorAction SilentlyContinue

    $item = Get-Item -LiteralPath $Executable
    Write-Host "`nГотово: Единственный бинарник создан -> $($item.FullName) ($([math]::Round($item.Length / 1MB, 2)) МБ)" -ForegroundColor Green
}

function Ensure-Binary {
    if (Test-BinaryNeedsBuild) {
        Build-OnePap
    }
}

function Show-RecentLogs([int]$Tail = 80) {
    foreach ($path in @($StdoutLog, $StderrLog)) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            Write-Host "`n--- $([System.IO.Path]::GetFileName($path)) ---" -ForegroundColor DarkGray
            Get-Content -LiteralPath $path -Tail $Tail
        }
    }
}

function ConvertTo-ProcessArgument([string]$Value) {
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Start-OnePap {
    $managed = Get-ManagedProcess
    if ($managed) {
        Write-Host "ONEPAP.24 уже запущен: PID=$($managed.Info.pid), $($managed.Info.url)" -ForegroundColor Yellow
        if (-not $NoBrowser) {
            Start-Process $managed.Info.url
        }
        return
    }

    Ensure-Binary
    $configPath = Resolve-ConfigPath
    New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
    Remove-Item -LiteralPath $RuntimeFile, $StdoutLog, $StderrLog -Force -ErrorAction SilentlyContinue

    $arguments = @(
        "-config", $configPath,
        "-runtime-file", $RuntimeFile
    )
    if (-not [string]::IsNullOrWhiteSpace($HostAddress)) {
        $arguments += @("-host", $HostAddress)
    }
    if ($Port -ge 0) {
        $arguments += @("-port", [string]$Port)
    }
    if ($StrictPort) {
        $arguments += "-strict-port"
    }
    if ($ServerOnly) {
        $arguments += "-server-only"
    }

    Write-Section "Запуск"
    if ($Foreground) {
        Push-Location $AppDir
        try {
            & $Executable @arguments
            if ($LASTEXITCODE -ne 0) {
                throw "Приложение завершилось с кодом $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }
        return
    }

    $argumentLine = ($arguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join " "
    $process = Start-Process `
        -FilePath $Executable `
        -ArgumentList $argumentLine `
        -WorkingDirectory $AppDir `
        -RedirectStandardOutput $StdoutLog `
        -RedirectStandardError $StderrLog `
        -WindowStyle Hidden `
        -PassThru

    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    $info = $null
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($process.HasExited) {
            Show-RecentLogs
            throw "Приложение завершилось при запуске с кодом $($process.ExitCode)"
        }
        $info = Read-RuntimeInfo
        if ($info) { break }
        Start-Sleep -Milliseconds 200
        $process.Refresh()
    }

    if (-not $info) {
        Show-RecentLogs
        throw "Сервер не создал runtime-файл. Проверьте журнал выше."
    }

    Write-Host "Запущено: PID=$($info.pid)" -ForegroundColor Green
    Write-Host "Адрес:   $($info.url)" -ForegroundColor Green
    if ($info.address -ne $info.requested_address) {
        Write-Host "Запрошенный порт был недоступен; приложение выбрало $($info.address)." -ForegroundColor Yellow
    }
    if (-not $NoBrowser) {
        Start-Process $info.url
    }
}

function Stop-OnePap {
    Write-Section "Остановка"
    $managed = Get-ManagedProcess
    if (-not $managed) {
        Write-Host "ONEPAP.24 не запущен."
        Remove-Item -LiteralPath $RuntimeFile -Force -ErrorAction SilentlyContinue
        return
    }

    $processId = [int]$managed.Info.pid
    Stop-Process -Id $processId -ErrorAction Stop
    try {
        Wait-Process -Id $processId -Timeout 10 -ErrorAction Stop
    }
    catch {
        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $RuntimeFile -Force -ErrorAction SilentlyContinue
    Write-Host "Остановлено: PID=$processId" -ForegroundColor Green
}

function Show-Status {
    Write-Section "Состояние"
    $managed = Get-ManagedProcess
    if (-not $managed) {
        Write-Host "Статус: остановлен" -ForegroundColor Yellow
        return
    }

    $info = $managed.Info
    Write-Host "Статус: запущен" -ForegroundColor Green
    Write-Host "PID:    $($info.pid)"
    Write-Host "URL:    $($info.url)"
    Write-Host "Порт:   $($info.port)"
    Write-Host "Старт:  $($info.started_at)"

    try {
        $response = Invoke-WebRequest -Uri "$($info.url)/api/v1/status" -TimeoutSec 3 -UseBasicParsing
        Write-Host "HTTP:   $($response.StatusCode)" -ForegroundColor Green
    }
    catch {
        Write-Host "HTTP:   нет ответа — $($_.Exception.Message)" -ForegroundColor Red
    }
}

function Open-OnePap {
    $managed = Get-ManagedProcess
    if (-not $managed) {
        throw "ONEPAP.24 не запущен. Выполните: .\onepap.ps1 start"
    }
    Start-Process $managed.Info.url
}

function Test-OnePap {
    Write-Section "Тесты"
    [void](Assert-Command "go")
    Invoke-InAppDirectory {
        go test ./... -count=1
    }
}

function Check-OnePap {
    Write-Section "Проверка качества"
    [void](Assert-Command "go")
    [void](Assert-Command "gofmt")

    Invoke-InAppDirectory {
        $unformatted = @(gofmt -l .)
        if ($unformatted.Count -gt 0) {
            $unformatted | ForEach-Object { Write-Host "Не отформатирован: $_" -ForegroundColor Red }
            throw "Запустите gofmt для перечисленных файлов"
        }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "go vet обнаружил ошибки" }
        go test ./... -race -count=1
    }
}

function Get-ConfiguredServerPort([string]$Path) {
    $inServer = $false
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -match '^\[(.+)\]$') {
            $inServer = $Matches[1] -eq "server"
            continue
        }
        if ($inServer -and $trimmed -match '^port\s*=\s*(\d+)') {
            return [int]$Matches[1]
        }
    }
    return 8080
}

function Test-LoopbackPort([int]$PortNumber) {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $PortNumber)
    try {
        $listener.Start()
        return [PSCustomObject]@{ Available = $true; Error = $null }
    }
    catch {
        return [PSCustomObject]@{ Available = $false; Error = $_.Exception.Message }
    }
    finally {
        try { $listener.Stop() } catch { }
    }
}

function Show-Doctor {
    Write-Section "Диагностика окружения"

    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        Write-Host "Go:        $(& go version)" -ForegroundColor Green
    }
    elseif (Test-Path -LiteralPath $BackendBinary) {
        Write-Host "Go:        не установлен, но готовый Go бинарник найден" -ForegroundColor Yellow
    }
    else {
        Write-Host "Go:        не установлен и бинарник отсутствует" -ForegroundColor Red
    }

    $wails = Get-Command wails3 -ErrorAction SilentlyContinue
    if ($wails) { Write-Host "Wails:     $(& wails3 version)" -ForegroundColor Green }
    elseif (Test-Path -LiteralPath $Executable) { Write-Host "Wails:     не установлен, но готовый Desktop EXE найден" -ForegroundColor Yellow }
    else { Write-Host "Wails:     не установлен" -ForegroundColor Red }

    $configPath = Resolve-ConfigPath
    Write-Host "Config:    $configPath"
    $configuredPort = if ($Port -ge 0) { $Port } else { Get-ConfiguredServerPort $configPath }
    if ($configuredPort -eq 0) {
        Write-Host "HTTP:      автоматический свободный порт" -ForegroundColor Green
    }
    else {
        $portCheck = Test-LoopbackPort $configuredPort
        if ($portCheck.Available) {
            Write-Host "HTTP:      127.0.0.1:$configuredPort доступен" -ForegroundColor Green
        }
        else {
            Write-Host "HTTP:      127.0.0.1:$configuredPort недоступен — $($portCheck.Error)" -ForegroundColor Red
            if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
                Get-NetTCPConnection -LocalPort $configuredPort -ErrorAction SilentlyContinue |
                    Select-Object LocalAddress, LocalPort, State, OwningProcess |
                    Format-Table -AutoSize
            }
            Write-Host "Исключённые Windows TCP-диапазоны:" -ForegroundColor Yellow
            & netsh interface ipv4 show excludedportrange protocol=tcp 2>$null
        }
    }

    $ports = @([System.IO.Ports.SerialPort]::GetPortNames() | Sort-Object)
    if ($ports.Count -gt 0) {
        Write-Host "COM:       $($ports -join ', ')" -ForegroundColor Green
    }
    else {
        Write-Host "COM:       последовательные порты не найдены" -ForegroundColor Yellow
    }

    $managed = Get-ManagedProcess
    if ($managed) {
        Write-Host "Desktop:   PID=$($managed.Info.pid), $($managed.Info.url)" -ForegroundColor Green
    }
    else {
        Write-Host "Desktop:   остановлен"
    }
}

function Clean-OnePap {
    Write-Section "Очистка"
    Stop-OnePap

    $paths = @(
        $RuntimeDir,
        $ArtifactsDir,
        (Join-Path $DesktopDir "target"),
        (Join-Path $AppDir "modbus-configurator.exe"),
        (Join-Path $AppDir "modbus-configurator.log"),
        (Join-Path $Root "modbus-configurator.log")
    )
    foreach ($path in $paths) {
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Recurse -Force
            Write-Host "Удалено: $path"
        }
    }

    Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Extension -in @(".pprof", ".out", ".coverprofile") } |
        Remove-Item -Force -ErrorAction SilentlyContinue

    if (Get-Command go -ErrorAction SilentlyContinue) {
        Invoke-InAppDirectory {
            go clean -cache -testcache
            if ($Deep) {
                go clean -modcache
            }
        }
    }

    Write-Host "Очистка завершена." -ForegroundColor Green
}

function Show-Help {
    @"
ONEPAP.24 — единый скрипт управления

  .\onepap.ps1 start                 собрать единый EXE и запустить Wails v3 окно
  .\onepap.ps1 start -ServerOnly     запустить только Go-сервер без окна браузера
  .\onepap.ps1 start -Port 0         выбрать свободный HTTP-порт автоматически
  .\onepap.ps1 start -Port 8083      запросить конкретный порт с безопасным fallback
  .\onepap.ps1 start -StrictPort     завершить запуск, если выбранный порт недоступен
  .\onepap.ps1 stop                  остановить управляемый экземпляр
  .\onepap.ps1 restart               перезапустить
  .\onepap.ps1 status                показать PID, URL и HTTP-состояние
  .\onepap.ps1 open                  открыть текущий UI
  .\onepap.ps1 build                 собрать единый Windows EXE (Go + Wails v3/WebView2)
  .\onepap.ps1 test                  запустить тесты
  .\onepap.ps1 check                 gofmt + vet + race tests
  .\onepap.ps1 doctor                проверить Go, Wails v3, HTTP-порт, COM-порты и runtime
  .\onepap.ps1 logs                  показать последние журналы
  .\onepap.ps1 clean                 удалить сборки, runtime, логи и кэши
  .\onepap.ps1 clean -Deep           дополнительно удалить кэш Go-модулей
"@ | Write-Host
}

try {
    switch ($Action) {
        "start"   { Start-OnePap }
        "stop"    { Stop-OnePap }
        "restart" { Stop-OnePap; Start-OnePap }
        "status"  { Show-Status }
        "open"    { Open-OnePap }
        "build"   { Build-OnePap }
        "test"    { Test-OnePap }
        "check"   { Check-OnePap }
        "clean"   { Clean-OnePap }
        "doctor"  { Show-Doctor }
        "logs"    { Show-RecentLogs }
        "help"    { Show-Help }
    }
}
catch {
    Write-Host "`nОшибка: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
