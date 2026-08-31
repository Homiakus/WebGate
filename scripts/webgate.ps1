<#
.SYNOPSIS
    Лаунчер менеджера проектов и компиляции WebGate.
.DESCRIPTION
    Запускает пайплайны компиляции WebGate, режим разработки, диагностику,
    тестирование качества (CI-parity) и упаковку дистрибутива через PowerShell.
.EXAMPLE
    .\scripts\webgate.ps1 doctor
    .\scripts\webgate.ps1 build --target all --release
    .\scripts\webgate.ps1 run server
    .\scripts\webgate.ps1 run dev
    .\scripts\webgate.ps1 test
    .\scripts\webgate.ps1 verify
    .\scripts\webgate.ps1 dist
#>

[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ManagerArgs
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$Manager = Join-Path $ScriptDir "project_manager.py"

# Поиск Python 3.11+
function Find-Python {
    $candidates = @(
        @{ Command = "py"; Prefix = @("-3") },
        @{ Command = "python3"; Prefix = @() },
        @{ Command = "python"; Prefix = @() }
    )

    foreach ($candidate in $candidates) {
        if (Get-Command $candidate.Command -ErrorAction SilentlyContinue) {
            try {
                $verOutput = & $candidate.Command @($candidate.Prefix) -c "import sys; print(f'{sys.version_info[0]}.{sys.version_info[1]}')" 2>$null
                if ($verOutput) {
                    $major, $minor = $verOutput.Trim().Split(".")
                    if ([int]$major -eq 3 -and [int]$minor -ge 11) {
                        return $candidate
                    }
                }
            }
            catch {}
        }
    }
    return $null
}

$python = Find-Python
if ($null -eq $python) {
    Write-Host @"
`e[31m[ОШИБКА] Для запуска менеджера проектов WebGate требуется Python 3.11+.`e[0m
Установите Python, затем перезапустите скрипт:
  winget install --id Python.Python.3.13 -e --source winget
"@ -ForegroundColor Red
    exit 1
}

# Ensure Cargo & Go are in PATH if installed in standard user locations
$CargoBin = Join-Path $env:USERPROFILE ".cargo\bin"
if ((Test-Path $CargoBin) -and ($env:PATH -notlike "*$CargoBin*")) {
    $env:PATH = "$CargoBin;$env:PATH"
}

$arguments = @()
$arguments += $python.Prefix
$arguments += $Manager
if ($ManagerArgs) {
    $arguments += $ManagerArgs
}

& $python.Command @arguments
exit $LASTEXITCODE
