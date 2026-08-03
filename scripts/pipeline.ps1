# Progressive Quality & Performance Pipeline for Modbus Configurator (PowerShell)
# Unified Orchestrator Script
[CmdletBinding()]
param(
    [ValidateSet("all", "audit", "benchmark", "profile", "stress", "compare")]
    [string]$Mode = "all",

    [int]$BenchCount = 5,
    [string]$BenchTime = "3s",
    [int]$StressCount = 20,
    [int]$StressParallel = 8,

    [string]$EvidenceDir = "..\audit\evidence",
    [string]$BaselineFile = "..\audit\evidence\benchmarks\baseline.txt",
    [string]$CurrentBenchFile = "..\audit\evidence\benchmarks\current.txt",
    [string]$ProfileDir = "..\audit\evidence\profiles"
)

$ErrorActionPreference = "Stop"
$WorkDir = Join-Path $PSScriptRoot "..\configurator"
Set-Location $WorkDir

function Print-Header([string]$Title) {
    Write-Host "`n========================================================" -ForegroundColor Cyan
    Write-Host " $Title " -ForegroundColor Cyan
    Write-Host "========================================================" -ForegroundColor Cyan
}

function Print-Step([int]$Current, [int]$Total, [string]$Desc) {
    Write-Host "`n[$Current/$Total] $Desc..." -ForegroundColor Yellow
}

function Ensure-Directory([string]$Path) {
    if (!(Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

$AbsEvidenceDir = [System.IO.Path]::GetFullPath((Join-Path $WorkDir $EvidenceDir))
$AbsProfileDir = [System.IO.Path]::GetFullPath((Join-Path $WorkDir $ProfileDir))
$AbsCurrentBenchFile = [System.IO.Path]::GetFullPath((Join-Path $WorkDir $CurrentBenchFile))
$AbsBaselineFile = [System.IO.Path]::GetFullPath((Join-Path $WorkDir $BaselineFile))

Ensure-Directory (Split-Path -Parent $AbsCurrentBenchFile)
Ensure-Directory $AbsProfileDir

# -----------------------------------------------------------------------------
# STAGE 1: ENVIRONMENT & TOOLCHAIN
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit") {
    Print-Header "STAGE 1: Environment & Toolchain Verification"
    Print-Step 1 9 "Checking Go compiler version and OS environment"
    go version
    go env GOOS GOARCH GOMAXPROCS
}

# -----------------------------------------------------------------------------
# STAGE 2: UNIT & INTEGRATION TESTS
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit") {
    Print-Header "STAGE 2: Unit & Integration Tests"
    Print-Step 2 9 "Running all package tests"
    go test ./... -v -count=1
    if ($LASTEXITCODE -ne 0) { Write-Error "Unit tests failed!" }
}

# -----------------------------------------------------------------------------
# STAGE 3: RACE CONDITION DETECTOR
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit") {
    Print-Header "STAGE 3: Concurrency Race Detection"
    Print-Step 3 9 "Executing test suite under -race detector"
    go test -race ./... -count=1
    if ($LASTEXITCODE -ne 0) { Write-Error "Race detector found data races!" }
}

# -----------------------------------------------------------------------------
# STAGE 4: STATIC ANALYSIS & VET
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit") {
    Print-Header "STAGE 4: Static Code Analysis"
    Print-Step 4 9 "Running go vet on all packages"
    go vet ./...
    if ($LASTEXITCODE -ne 0) { Write-Error "go vet reported issues!" }
}

# -----------------------------------------------------------------------------
# STAGE 5: BENCHMARKS & MEMORY ALLOCATION
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit" -or $Mode -eq "benchmark") {
    Print-Header "STAGE 5: Benchmarks & Memory Allocation Analysis"
    Print-Step 5 9 "Executing benchmarks (Count: $BenchCount, BenchTime: $BenchTime)"
    go test ./... -run '^$' -bench . -benchmem -count $BenchCount -benchtime $BenchTime | Tee-Object -FilePath $AbsCurrentBenchFile
    if ($LASTEXITCODE -ne 0) { Write-Error "Benchmarks failed!" }
    Write-Host "Benchmark raw output saved to: $AbsCurrentBenchFile" -ForegroundColor Green
}

# -----------------------------------------------------------------------------
# STAGE 6: ESCAPE ANALYSIS
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "audit") {
    Print-Header "STAGE 6: Compiler Escape Analysis (-gcflags)"
    Print-Step 6 9 "Analyzing heap allocations and escape optimization"
    go test -gcflags="-m=2" ./...
    if ($LASTEXITCODE -ne 0) { Write-Error "Escape analysis failed!" }
}

# -----------------------------------------------------------------------------
# STAGE 7: CPU & HEAP PROFILING
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "profile") {
    Print-Header "STAGE 7: CPU & Memory Heap Profiling"
    Print-Step 7 9 "Generating pprof files (cpu.pprof, mem.pprof)"
    $CpuFile = Join-Path $AbsProfileDir "cpu.pprof"
    $MemFile = Join-Path $AbsProfileDir "mem.pprof"
    go test . -run '^$' -bench . -benchmem -cpuprofile $CpuFile -memprofile $MemFile -benchtime 3s
    if ($LASTEXITCODE -ne 0) { Write-Error "Profiling failed!" }
    Write-Host "Profiles saved to:`n  CPU: $CpuFile`n  Mem: $MemFile" -ForegroundColor Green
}

# -----------------------------------------------------------------------------
# STAGE 8: STRESS & PARALLEL STABILITY TESTING
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "stress") {
    Print-Header "STAGE 8: Concurrency Stress & Shuffle Stability"
    Print-Step 8 9 "Running stress loop (Count: $StressCount, Parallel: $StressParallel, Shuffle: ON)"
    go test ./... -race -count $StressCount -parallel $StressParallel -shuffle on
    if ($LASTEXITCODE -ne 0) { Write-Error "Stress test failed!" }
    Write-Host "Stress test PASSED cleanly with 0 data races!" -ForegroundColor Green
}

# -----------------------------------------------------------------------------
# STAGE 9: BENCHMARK COMPARISON
# -----------------------------------------------------------------------------
if ($Mode -eq "all" -or $Mode -eq "compare") {
    Print-Header "STAGE 9: Benchmark Comparison vs Baseline"
    Print-Step 9 9 "Comparing current benchmark results with baseline"

    if ((Test-Path $AbsBaselineFile) -and (Test-Path $AbsCurrentBenchFile)) {
        if (Get-Command benchstat -ErrorAction SilentlyContinue) {
            benchstat $AbsBaselineFile $AbsCurrentBenchFile
        } else {
            Write-Host "benchstat utility not installed. Raw file summary:" -ForegroundColor Yellow
            Write-Host "--- Baseline ($AbsBaselineFile) ---" -ForegroundColor Gray
            Get-Content $AbsBaselineFile -ErrorAction SilentlyContinue | Select-Object -First 20
            Write-Host "--- Current ($AbsCurrentBenchFile) ---" -ForegroundColor Gray
            Get-Content $AbsCurrentBenchFile -ErrorAction SilentlyContinue | Select-Object -First 20
        }
    } else {
        Write-Host "Notice: Baseline file ($AbsBaselineFile) not found for comparison. Saving current as baseline recommendation." -ForegroundColor Yellow
    }
}

Print-Header "PROGRESSIVE PIPELINE COMPLETED SUCCESSFULLY!"
