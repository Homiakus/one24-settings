# Audit Orchestrator Script (Delegates to Progressive Pipeline)
[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\pipeline.ps1" -Mode "audit"
