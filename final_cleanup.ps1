# Final Cleanup and Index Rebuild Script

$ErrorActionPreference = "Stop"
$DB = "config.db"

Write-Host "🔧 Repariere Indizes und Foreign Keys..." -ForegroundColor Cyan

# Backup vor finaler Reparatur
$finalBackup = "config.db.before_final_fix"
Copy-Item $DB $finalBackup -Force
Write-Host "✅ Backup erstellt: $finalBackup" -ForegroundColor Green

# Erstelle Reparatur-Script
$repairSQL = @"
-- Disable foreign keys temporarily
PRAGMA foreign_keys = OFF;

-- Rebuild all indices
REINDEX;

-- Analyze database
ANALYZE;

-- Re-enable foreign keys
PRAGMA foreign_keys = ON;

-- Vacuum to cleanup
VACUUM;
"@

$repairSQL | Out-File -FilePath "repair_indices.sql" -Encoding UTF8

# Führe Reparatur aus
Write-Host "Führe Reparatur aus..." -ForegroundColor Yellow
sqlite3 $DB ".read repair_indices.sql" 2>&1

# Verifiziere
Write-Host "`nVerifiziere Reparatur..." -ForegroundColor Yellow
$check = sqlite3 $DB "PRAGMA integrity_check;" 2>&1

if ($check -eq "ok") {
    Write-Host "✅ Datenbank Integrität: OK" -ForegroundColor Green
} else {
    Write-Host "⚠ Integritätsprüfung:" -ForegroundColor Yellow
    Write-Host $check -ForegroundColor Gray
}

# Teste alle Tabellen
Write-Host "`nTeste alle Tabellen..." -ForegroundColor Yellow
$tables = @("users", "ai_models", "traders", "user_signal_sources", "system_config", "beta_codes", "deals", "deal_events", "exchanges")

foreach ($table in $tables) {
    $count = sqlite3 $DB "SELECT COUNT(*) FROM $table;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ $table : $count Einträge" -ForegroundColor Green
    } else {
        Write-Host "❌ $table : Fehler" -ForegroundColor Red
    }
}

Write-Host "`n✅ Reparatur abgeschlossen!" -ForegroundColor Green
Write-Host "   Die Datenbank sollte jetzt vollständig funktionieren." -ForegroundColor Gray
