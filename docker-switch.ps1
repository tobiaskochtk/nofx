# Quick Docker Context Switcher
# Schneller Wechsel zwischen Docker-Hosts

param(
    [Parameter(Position=0)]
    [ValidateSet('local', 'proxmox', 'list', 'status')]
    [string]$Target = 'list'
)

function Show-CurrentContext {
    $current = docker context show
    $contexts = docker context ls --format "{{.Name}}`t{{.DockerEndpoint}}`t{{.Current}}" | ConvertFrom-Csv -Delimiter "`t" -Header "Name","Endpoint","Current"
    
    Write-Host "`n📍 Aktueller Docker Context:" -ForegroundColor Cyan
    Write-Host "   $current" -ForegroundColor Green -NoNewline
    
    $endpoint = ($contexts | Where-Object { $_.Name -eq $current }).Endpoint
    Write-Host " → $endpoint" -ForegroundColor Gray
    
    Write-Host "`n📋 Verfügbare Contexts:" -ForegroundColor Yellow
    $contexts | ForEach-Object {
        $marker = if ($_.Name -eq $current) { "→" } else { " " }
        $color = if ($_.Name -eq $current) { "Green" } else { "White" }
        Write-Host "  $marker $($_.Name)" -ForegroundColor $color -NoNewline
        Write-Host " ($($_.Endpoint))" -ForegroundColor Gray
    }
    Write-Host ""
}

switch ($Target) {
    'local' {
        Write-Host "🔄 Wechsle zu lokalem Docker..." -ForegroundColor Cyan
        docker context use default
        Show-CurrentContext
    }
    'proxmox' {
        Write-Host "🔄 Wechsle zu Proxmox Docker..." -ForegroundColor Cyan
        docker context use proxmox
        Show-CurrentContext
    }
    'status' {
        Show-CurrentContext
    }
    default {
        Show-CurrentContext
        Write-Host "💡 Verwendung:" -ForegroundColor Yellow
        Write-Host "   .\docker-switch.ps1 local    - Wechsel zu lokalem Docker"
        Write-Host "   .\docker-switch.ps1 proxmox  - Wechsel zu Proxmox Docker"
        Write-Host "   .\docker-switch.ps1 status   - Zeige aktuellen Context"
        Write-Host ""
    }
}
