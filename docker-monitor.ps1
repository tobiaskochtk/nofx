# Docker Multi-Host Monitor
# Überwacht lokalen Windows Docker und Proxmox Docker gleichzeitig

param(
    [switch]$Watch,
    [int]$RefreshSeconds = 5
)

function Get-DockerInfo {
    param([string]$Context, [string]$Name)
    
    Write-Host "`n╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║  $Name" -ForegroundColor Cyan
    Write-Host "╚═══════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
    
    try {
        # Container Status
        $containers = docker --context $Context ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}\t{{.Ports}}" 2>$null
        
        if ($LASTEXITCODE -eq 0) {
            Write-Host "`n📦 Running Containers:" -ForegroundColor Green
            $containers | ForEach-Object { 
                if ($_ -match "NAMES") {
                    Write-Host $_ -ForegroundColor Yellow
                } else {
                    Write-Host $_
                }
            }
            
            # System Info
            $info = docker --context $Context info --format "OS: {{.OperatingSystem}} | Docker: {{.ServerVersion}} | Containers: {{.Containers}} ({{.ContainersRunning}} running)" 2>$null
            if ($LASTEXITCODE -eq 0) {
                Write-Host "`n💻 System Info:" -ForegroundColor Magenta
                Write-Host "   $info"
            }
            
            # Resource Usage (nur für lokalen Docker)
            if ($Context -eq "default") {
                $stats = docker --context $Context stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" 2>$null
                if ($LASTEXITCODE -eq 0 -and $stats) {
                    Write-Host "`n📊 Resource Usage:" -ForegroundColor Blue
                    $stats | ForEach-Object { 
                        if ($_ -match "NAME") {
                            Write-Host $_ -ForegroundColor Yellow
                        } else {
                            Write-Host $_
                        }
                    }
                }
            }
        } else {
            Write-Host "❌ Verbindung zu $Name fehlgeschlagen" -ForegroundColor Red
        }
    } catch {
        Write-Host "❌ Error: $($_.Exception.Message)" -ForegroundColor Red
    }
}

function Show-AllDockerHosts {
    Clear-Host
    Write-Host "╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor White
    Write-Host "║           Docker Multi-Host Monitor                           ║" -ForegroundColor White
    Write-Host "║           $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')                         ║" -ForegroundColor White
    Write-Host "╚═══════════════════════════════════════════════════════════════╝" -ForegroundColor White
    
    # Lokaler Docker
    Get-DockerInfo -Context "default" -Name "🏠 Lokaler Windows Docker"
    
    # Proxmox Docker
    Get-DockerInfo -Context "proxmox" -Name "🌐 Proxmox Docker (192.168.188.163)"
    
    Write-Host "`n" -NoNewline
    Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Gray
    
    if ($Watch) {
        Write-Host "⏱️  Aktualisierung in $RefreshSeconds Sekunden... (Strg+C zum Beenden)" -ForegroundColor Gray
    }
}

# Hauptlogik
if ($Watch) {
    while ($true) {
        Show-AllDockerHosts
        Start-Sleep -Seconds $RefreshSeconds
    }
} else {
    Show-AllDockerHosts
}
