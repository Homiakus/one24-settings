# Profiling Script (Delegates to Progressive Pipeline)
[CmdletBinding()]
param(
    [string]$OutputDir = "..\audit\evidence\profiles"
)
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\pipeline.ps1" -Mode "profile" -ProfileDir $OutputDir
