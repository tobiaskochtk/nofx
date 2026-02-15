# Complete Database Reset Script
# Löscht WAL-Dateien und erstellt saubere DB

$ErrorActionPreference = "Stop"
$TIMESTAMP = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "🔧 Vollständiger Database Reset" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Gray

# Schritt 1: Backup der aktuellen Situation
Write-Host "`n[1/4] Erstelle Backup..." -ForegroundColor Yellow
if (Test-Path "config.db") {
    Copy-Item "config.db" "config.db.backup_before_reset_$TIMESTAMP" -Force
    Write-Host "✅ DB gesichert" -ForegroundColor Green
}

# Schritt 2: Lösche alle WAL-Dateien
Write-Host "`n[2/4] Lösche WAL-Dateien..." -ForegroundColor Yellow
$walFiles = @("config.db-wal", "config.db-shm")
foreach ($file in $walFiles) {
    if (Test-Path $file) {
        Remove-Item $file -Force
        Write-Host "   ✓ Gelöscht: $file" -ForegroundColor Green
    }
}

# Schritt 3: Verwende die reparierte DB
Write-Host "`n[3/4] Installiere reparierte Datenbank..." -ForegroundColor Yellow
if (Test-Path "config.db.repaired") {
    # Lösche alte config.db
    Remove-Item "config.db" -Force -ErrorAction SilentlyContinue
    
    # Kopiere reparierte DB
    Copy-Item "config.db.repaired" "config.db" -Force
    Write-Host "✅ Reparierte DB installiert" -ForegroundColor Green
} else {
    Write-Host "❌ config.db.repaired nicht gefunden!" -ForegroundColor Red
    Write-Host "   Verwende Backup: config.db.backup_20251113191521" -ForegroundColor Yellow
    
    Remove-Item "config.db" -Force -ErrorAction SilentlyContinue
    Copy-Item "config.db.backup_20251113191521" "config.db" -Force
    Write-Host "✅ Backup wiederhergestellt" -ForegroundColor Green
}

# Schritt 4: Verifiziere und initialisiere WAL-Modus neu
Write-Host "`n[4/4] Initialisiere WAL-Modus..." -ForegroundColor Yellow

$initSQL = @"
-- Re-initialize WAL mode
PRAGMA journal_mode=DELETE;
PRAGMA journal_mode=WAL;
PRAGMA synchronous=FULL;
PRAGMA integrity_check;
"@

$initSQL | Out-File -FilePath "init_wal.sql" -Encoding UTF8
$result = sqlite3 config.db ".read init_wal.sql" 2>&1

Write-Host "Ergebnis:" -ForegroundColor Gray
Write-Host $result -ForegroundColor Gray

# Teste alle Tabellen
Write-Host "`n[Verifikation] Teste alle Tabellen..." -ForegroundColor Yellow
$tables = @("users", "ai_models", "traders", "exchanges", "system_config", "deals", "deal_events")

$allGood = $true
foreach ($table in $tables) {
    $count = sqlite3 config.db "SELECT COUNT(*) FROM $table;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ $table : $count Einträge" -ForegroundColor Green
    } else {
        Write-Host "❌ $table : Fehler beim Lesen" -ForegroundColor Red
        $allGood = $false
    }
}

Write-Host "`n" + ("=" * 70) -ForegroundColor Gray
if ($allGood) {
    Write-Host "✅ DATABASE RESET ERFOLGREICH!" -ForegroundColor Green
    Write-Host "`nDie Datenbank sollte jetzt funktionieren." -ForegroundColor White
    Write-Host "Starte deine Anwendung neu." -ForegroundColor White
} else {
    Write-Host "⚠️  EINIGE PROBLEME GEFUNDEN" -ForegroundColor Yellow
    Write-Host "`nMöglicherweise ist eine manuelle Intervention nötig." -ForegroundColor Yellow
}
Write-Host ("=" * 70) -ForegroundColor Gray
