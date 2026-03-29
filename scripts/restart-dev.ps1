$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$webRoot = Join-Path $repoRoot 'web'
$logsDir = Join-Path $repoRoot 'logs'
$backendLog = Join-Path $logsDir 'dev-backend-3000.log'
$frontendLog = Join-Path $logsDir 'dev-frontend-5173.log'

function Assert-Command {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Name
  )

  Get-Command $Name -ErrorAction Stop | Out-Null
}

function Stop-ListeningProcesses {
  param(
    [Parameter(Mandatory = $true)]
    [int[]]$Ports
  )

  $processIds = @()
  foreach ($port in $Ports) {
    $connections = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
    if ($connections) {
      $processIds += $connections | Select-Object -ExpandProperty OwningProcess
    }
  }

  $processIds = $processIds | Sort-Object -Unique
  foreach ($processId in $processIds) {
    Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
  }
}

function Wait-ForPort {
  param(
    [Parameter(Mandatory = $true)]
    [int]$Port,
    [int]$TimeoutSeconds = 60
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
      return
    }
    Start-Sleep -Milliseconds 500
  }

  throw "Port $Port did not become ready within $TimeoutSeconds seconds."
}

Assert-Command -Name 'go'
Assert-Command -Name 'bun'

New-Item -ItemType Directory -Path $logsDir -Force | Out-Null

Stop-ListeningProcesses -Ports @(3000, 5173)
Start-Sleep -Seconds 2

Set-Content -Path $backendLog -Value ''
Set-Content -Path $frontendLog -Value ''

$backendCommand = "Set-Location '$repoRoot'; go run main.go *> '$backendLog'"
$frontendCommand = "Set-Location '$webRoot'; bun run dev *> '$frontendLog'"

Start-Process -FilePath 'powershell.exe' -ArgumentList @(
  '-NoLogo',
  '-NoProfile',
  '-ExecutionPolicy',
  'Bypass',
  '-Command',
  $backendCommand
) -WindowStyle Hidden | Out-Null

Start-Process -FilePath 'powershell.exe' -ArgumentList @(
  '-NoLogo',
  '-NoProfile',
  '-ExecutionPolicy',
  'Bypass',
  '-Command',
  $frontendCommand
) -WindowStyle Hidden | Out-Null

Wait-ForPort -Port 3000
Wait-ForPort -Port 5173

Write-Output 'Frontend: http://127.0.0.1:5173/'
Write-Output 'Portal:   http://127.0.0.1:5173/token-portal/login'
Write-Output 'Backend:  http://127.0.0.1:3000/'
