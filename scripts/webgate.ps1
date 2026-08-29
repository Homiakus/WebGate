param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ManagerArgs
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Manager = Join-Path $ScriptDir "project_manager.py"

function Find-Python {
    $candidates = @(
        @{ Command = "py"; Prefix = @("-3") },
        @{ Command = "python3"; Prefix = @() },
        @{ Command = "python"; Prefix = @() }
    )

    foreach ($candidate in $candidates) {
        if (Get-Command $candidate.Command -ErrorAction SilentlyContinue) {
            return $candidate
        }
    }
    return $null
}

$python = Find-Python
if ($null -eq $python) {
    Write-Error @"
Python 3.11+ is required to launch the WebGate project manager.
Install it once, then rerun this script. On Windows with winget:
  winget install --id Python.Python.3.13 -e --source winget
"@
    exit 1
}

$arguments = @()
$arguments += $python.Prefix
$arguments += $Manager
$arguments += $ManagerArgs

& $python.Command @arguments
exit $LASTEXITCODE
