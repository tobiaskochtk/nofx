# n8n Debug Script
# Testet alle n8n-Endpunkte und Konfigurationen

param(
    [string]$N8nHost = "192.168.188.163",
    [int]$Port = 5678
)

$baseUrl = "http://${N8nHost}:${Port}"

Write-Host "`n╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║  n8n Debug Tool - $baseUrl" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════════════════╝`n" -ForegroundColor Cyan

# Test 1: Health Check
Write-Host "📡 Test 1: Health Check..." -ForegroundColor Yellow
try {
    $health = Invoke-WebRequest -Uri "$baseUrl/healthz" -TimeoutSec 5
    Write-Host "   ✅ Status: $($health.StatusCode) - $($health.Content)" -ForegroundColor Green
} catch {
    Write-Host "   ❌ Health check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 2: Frontend
Write-Host "`n🌐 Test 2: Frontend (HTML)..." -ForegroundColor Yellow
try {
    $frontend = Invoke-WebRequest -Uri "$baseUrl/" -TimeoutSec 5
    Write-Host "   ✅ Status: $($frontend.StatusCode)" -ForegroundColor Green
    $hasApp = $frontend.Content -match 'id="app"'
    Write-Host "   $(if($hasApp){'✅'}else{'❌'}) App div found: $hasApp" -ForegroundColor $(if($hasApp){'Green'}else{'Red'})
} catch {
    Write-Host "   ❌ Frontend failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 3: REST API
Write-Host "`n🔐 Test 3: REST API (Login endpoint)..." -ForegroundColor Yellow
try {
    $rest = Invoke-WebRequest -Uri "$baseUrl/rest/login" -Method GET -TimeoutSec 5 -ErrorAction SilentlyContinue
    Write-Host "   ✅ Status: $($rest.StatusCode)" -ForegroundColor Green
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "   ✅ Status: 401 (Expected - Auth required)" -ForegroundColor Green
    } else {
        Write-Host "   ❌ REST API failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 4: Port Check
Write-Host "`n🔌 Test 4: Port Connectivity..." -ForegroundColor Yellow
$portTest = Test-NetConnection -ComputerName $N8nHost -Port $Port -WarningAction SilentlyContinue
$portStatus = if($portTest.TcpTestSucceeded){'Open'}else{'Closed'}
$portEmoji = if($portTest.TcpTestSucceeded){'✅'}else{'❌'}
$portColor = if($portTest.TcpTestSucceeded){'Green'}else{'Red'}
Write-Host "   $portEmoji Port ${Port}: $portStatus" -ForegroundColor $portColor

# Test 5: Docker Container Status
Write-Host "`n🐳 Test 5: Docker Container Status..." -ForegroundColor Yellow
try {
    $container = docker --context proxmox inspect n8n-n8n-1 --format '{{.State.Status}}' 2>$null
    Write-Host "   ✅ Container Status: $container" -ForegroundColor Green
    
    $env = docker --context proxmox exec n8n-n8n-1 env 2>$null | Select-String "N8N_HOST|N8N_PROTOCOL|N8N_PORT|N8N_PUSH"
    Write-Host "   📋 Environment:" -ForegroundColor Cyan
    $env | ForEach-Object { Write-Host "      $_" -ForegroundColor Gray }
} catch {
    Write-Host "   ❌ Docker check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 6: Container Logs (letzte Fehler)
Write-Host "`n📝 Test 6: Recent Container Logs..." -ForegroundColor Yellow
try {
    $logs = docker --context proxmox logs n8n-n8n-1 --tail 10 2>$null | Select-String "error|Error|failed|Failed" -Context 1
    if ($logs) {
        Write-Host "   ⚠️  Errors found in logs:" -ForegroundColor Yellow
        $logs | ForEach-Object { Write-Host "      $_" -ForegroundColor Red }
    } else {
        Write-Host "   ✅ No recent errors in logs" -ForegroundColor Green
    }
} catch {
    Write-Host "   ❌ Could not read logs: $($_.Exception.Message)" -ForegroundColor Red
}

# Zusammenfassung
Write-Host "`n╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor White
Write-Host "║  Zusammenfassung                                              ║" -ForegroundColor White
Write-Host "╚═══════════════════════════════════════════════════════════════╝" -ForegroundColor White
Write-Host "`n💡 Nächste Schritte wenn Probleme bestehen:" -ForegroundColor Yellow
Write-Host "   1. Browser-Cache leeren (Ctrl+Shift+Delete)" -ForegroundColor Gray
Write-Host "   2. Inkognito-Modus versuchen (Ctrl+Shift+N)" -ForegroundColor Gray
Write-Host "   3. Browser Console öffnen (F12) und Fehler prüfen" -ForegroundColor Gray
Write-Host "   4. WebSocket-Verbindung im Network-Tab prüfen" -ForegroundColor Gray
Write-Host "`n🌐 URL: $baseUrl`n" -ForegroundColor Cyan
