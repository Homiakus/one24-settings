# Benchmark Runner Script (Delegates to Progressive Pipeline)
[CmdletBinding()]
param(
    [int]$Count = 10,
    [string]$BenchTime = "3s"
)
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\pipeline.ps1" -Mode "benchmark" -BenchCount $Count -BenchTime $BenchTime
