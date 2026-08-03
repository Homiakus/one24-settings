# Compare Benchmarks Script (Delegates to Progressive Pipeline)
[CmdletBinding()]
param(
    [string]$BaselineFile = "..\audit\evidence\benchmarks\baseline.txt",
    [string]$NewFile = "..\audit\evidence\benchmarks\current.txt"
)
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\pipeline.ps1" -Mode "compare" -BaselineFile $BaselineFile -CurrentBenchFile $NewFile
