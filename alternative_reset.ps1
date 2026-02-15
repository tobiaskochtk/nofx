# Alternative Reset - Erstelle komplett neue DB ohne alte Dateien zu löschen

$ErrorActionPreference = "Stop"
$TIMESTAMP = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "🔧 Alternative Database Reset" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Gray

# Schritt 1: Benenne alte Dateien um
Write-Host "`n[1/3] Sichere alte Dateien..." -ForegroundColor Yellow
$oldFiles = @("config.db", "config.db-wal", "config.db-shm")
foreach ($file in $oldFiles) {
    if (Test-Path $file) {
        $newName = "$file.locked_$TIMESTAMP"
        try {
            Rename-Item $file $newName -Force -ErrorAction Stop
            Write-Host "   ✓ Umbenannt: $file -> $newName" -ForegroundColor Green
        } catch {
            Write-Host "   ⚠ Konnte $file nicht umbenennen (möglicherweise gesperrt)" -ForegroundColor Yellow
        }
    }
}

# Warte kurz
Start-Sleep -Seconds 1

# Schritt 2: Erstelle neue saubere DB
Write-Host "`n[2/3] Erstelle neue Datenbank..." -ForegroundColor Yellow

if (-not (Test-Path "config.db")) {
    if (Test-Path "config.db.repaired") {
        Copy-Item "config.db.repaired" "config.db" -Force
        Write-Host "✅ Neue DB von config.db.repaired erstellt" -ForegroundColor Green
    } else {
        Write-Host "❌ config.db.repaired nicht gefunden!" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "⚠️  config.db existiert noch (gesperrt)" -ForegroundColor Yellow
    Write-Host "   Bitte stoppe alle Prozesse die die DB verwenden und versuche es erneut." -ForegroundColor Yellow
    exit 1
}

# Schritt 3: Initialisiere WAL-Modus
Write-Host "`n[3/3] Initialisiere neuen WAL-Modus..." -ForegroundColor Yellow

$initSQL = @"
PRAGMA journal_mode=WAL;
PRAGMA synchronous=FULL;
PRAGMA busy_timeout=5000;
"@

$initSQL | Out-File -FilePath "init_new_db.sql" -Encoding UTF8
$result = sqlite3 config.db ".read init_new_db.sql" 2>&1

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
        Write-Host "❌ $table : Fehler" -ForegroundColor Red
        $allGood = $false
    }
}

# Zeige neue WAL-Dateien
Write-Host "`n[Info] Neue WAL-Dateien:" -ForegroundColor Cyan
Get-ChildItem config.db, config.db-wal, config.db-shm -ErrorAction SilentlyContinue | 
    Select-Object Name, @{N='Size';E={'{0:N0} bytes' -f $_.Length}}, LastWriteTime |
    Format-Table -AutoSize

Write-Host ("=" * 70) -ForegroundColor Gray
if ($allGood) {
    Write-Host "✅ DATABASE RESET ERFOLGREICH!" -ForegroundColor Green
    Write-Host "`nDie neue Datenbank ist bereit." -ForegroundColor White
    Write-Host "Die alten Dateien wurden umbenannt zu *.locked_$TIMESTAMP" -ForegroundColor Gray
    Write-Host "`nStarte deine Anwendung neu:" -ForegroundColor White
    Write-Host "   docker compose up -d" -ForegroundColor Gray
} else {
    Write-Host "⚠️  PROBLEME GEFUNDEN" -ForegroundColor Yellow
}
Write-Host ("=" * 70) -ForegroundColor Gray
