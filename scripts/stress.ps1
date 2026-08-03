# Stress Test Script (Delegates to Progressive Pipeline)
[CmdletBinding()]
param(
    [int]$Count = 50,
    [int]$Parallel = 8
)
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\pipeline.ps1" -Mode "stress" -StressCount $Count -StressParallel $Parallel
