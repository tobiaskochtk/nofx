param(
    [int]$RetentionHours = 168,
    [switch]$StartDockerIfNeeded,
    [switch]$ShutdownIfStarted
)

$ErrorActionPreference = "Stop"

function Test-DockerReady {
    try {
        docker version | Out-Null
        return $LASTEXITCODE -eq 0
    } catch {
        return $false
    }
}

$dockerDesktopPath = Join-Path $env:ProgramFiles "Docker\Docker\Docker Desktop.exe"
$dockerCliPath = Join-Path $env:ProgramFiles "Docker\Docker\DockerCli.exe"
$startedDocker = $false

if (-not (Test-DockerReady)) {
    if (-not $StartDockerIfNeeded) {
        Write-Host "Docker is not running. Nothing to prune."
        exit 0
    }

    if (-not (Test-Path $dockerDesktopPath)) {
        throw "Docker Desktop executable not found at $dockerDesktopPath"
    }

    Write-Host "Starting Docker Desktop..."
    Start-Process $dockerDesktopPath | Out-Null
    $ready = $false

    for ($i = 0; $i -lt 120; $i++) {
        Start-Sleep -Seconds 2
        if (Test-DockerReady) {
            $ready = $true
            $startedDocker = $true
            break
        }
    }

    if (-not $ready) {
        throw "Docker did not become ready in time."
    }
}

$ageFilter = "until=${RetentionHours}h"

Write-Host "Docker usage before cleanup:"
docker system df

Write-Host ""
Write-Host "Pruning build cache older than $RetentionHours hours..."
docker builder prune -af --filter $ageFilter

Write-Host ""
Write-Host "Pruning unused images older than $RetentionHours hours..."
docker image prune -af --filter $ageFilter

Write-Host ""
Write-Host "Docker usage after cleanup:"
docker system df

if ($startedDocker -and $ShutdownIfStarted -and (Test-Path $dockerCliPath)) {
    Write-Host ""
    Write-Host "Shutting Docker Desktop down..."
    & $dockerCliPath -Shutdown | Out-Null
}
