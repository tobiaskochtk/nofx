# ====================================================================
# Database Repair Script V2 - Robuste Methode mit direktem SQL Dump
# ====================================================================

$ErrorActionPreference = "Stop"
$CURRENT_DB = "config.db"
$BACKUP_DB = "config.db.corrupt_1762854593.db"
$NEW_DB = "config.db.repaired"
$TIMESTAMP = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "🔧 Database Repair Tool V2 - Start" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Gray

# ====================================================================
# Schritt 1: Backup erstellen
# ====================================================================
Write-Host "`n[1/5] Erstelle Sicherheitskopie..." -ForegroundColor Yellow
$backupName = "config.db.backup_$TIMESTAMP"
Copy-Item $CURRENT_DB $backupName
Write-Host "✅ Backup erstellt: $backupName" -ForegroundColor Green

# ====================================================================
# Schritt 2: Prüfe Backup-Datenbank
# ====================================================================
Write-Host "`n[2/5] Prüfe Backup-Datenbank..." -ForegroundColor Yellow
$exchangeCount = sqlite3 $BACKUP_DB "SELECT COUNT(*) FROM exchanges;" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Backup exchanges Tabelle ist auch korrupt!" -ForegroundColor Red
    exit 1
}
Write-Host "✅ Backup exchanges Tabelle: $exchangeCount Einträge" -ForegroundColor Green

# ====================================================================
# Schritt 3: Exportiere alle Tabellen außer exchanges mit SQLite dump
# ====================================================================
Write-Host "`n[3/5] Exportiere aktuelle Daten..." -ForegroundColor Yellow

# Erstelle Dump-Script das exchanges ausschließt
$dumpScript = @"
.output export_current_data.sql
.mode insert
.dump users
.dump ai_models
.dump traders
.dump user_signal_sources
.dump system_config
.dump beta_codes
.dump deals
.dump deal_events
.output stdout
"@

$dumpScript | Out-File -FilePath "dump_current.txt" -Encoding ASCII
sqlite3 $CURRENT_DB ".read dump_current.txt" 2>&1

if (Test-Path "export_current_data.sql") {
    $size = (Get-Item "export_current_data.sql").Length
    Write-Host "✅ Export erfolgreich: $('{0:N0}' -f $size) Bytes" -ForegroundColor Green
} else {
    Write-Host "❌ Export fehlgeschlagen!" -ForegroundColor Red
    exit 1
}

# ====================================================================
# Schritt 4: Exportiere exchanges aus Backup
# ====================================================================
Write-Host "`n[4/5] Exportiere exchanges aus Backup..." -ForegroundColor Yellow

$exchangesDumpScript = @"
.output export_exchanges_backup.sql
.mode insert
.dump exchanges
.output stdout
"@

$exchangesDumpScript | Out-File -FilePath "dump_exchanges.txt" -Encoding ASCII
sqlite3 $BACKUP_DB ".read dump_exchanges.txt" 2>&1

if (Test-Path "export_exchanges_backup.sql") {
    $size = (Get-Item "export_exchanges_backup.sql").Length
    Write-Host "✅ Exchanges Export erfolgreich: $('{0:N0}' -f $size) Bytes" -ForegroundColor Green
} else {
    Write-Host "❌ Exchanges Export fehlgeschlagen!" -ForegroundColor Red
    exit 1
}

# ====================================================================
# Schritt 5: Erstelle neue Datenbank
# ====================================================================
Write-Host "`n[5/5] Erstelle neue reparierte Datenbank..." -ForegroundColor Yellow

# Lösche alte reparierte DB
if (Test-Path $NEW_DB) {
    Remove-Item $NEW_DB -Force
}

# Hole Schema
Write-Host "   Exportiere Schema..." -ForegroundColor Gray
sqlite3 $CURRENT_DB ".schema" | Out-File -FilePath "schema.sql" -Encoding UTF8

# Entferne problematische Einträge
$schema = Get-Content "schema.sql" | Where-Object {
    $_ -notmatch "^PRAGMA" -and
    $_ -notmatch "sqlite_sequence" -and
    $_.Trim() -ne ""
}
$schema | Out-File -FilePath "schema_clean.sql" -Encoding UTF8

# Erstelle neue DB mit Schema
Write-Host "   Erstelle Schema..." -ForegroundColor Gray
sqlite3 $NEW_DB ".read schema_clean.sql" 2>&1
Write-Host "   ✓ Schema erstellt" -ForegroundColor Green

# Importiere Daten - kombiniere beide Exports
Write-Host "   Importiere alle Daten..." -ForegroundColor Gray

# Kombiniere Exports
$combinedImport = @"
-- Combined import script
BEGIN TRANSACTION;

-- Import current data (all tables except exchanges)
"@

$currentData = Get-Content "export_current_data.sql" -Raw
$combinedImport += "`n$currentData`n"

$combinedImport += @"

-- Import exchanges from backup
"@

$exchangesData = Get-Content "export_exchanges_backup.sql" -Raw
$combinedImport += "`n$exchangesData`n"

$combinedImport += @"

COMMIT;
"@

$combinedImport | Out-File -FilePath "combined_import.sql" -Encoding UTF8

# Führe Import aus
sqlite3 $NEW_DB ".read combined_import.sql" 2>&1

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Import fehlgeschlagen!" -ForegroundColor Red
    Write-Host "   Versuche Import ohne Transaction..." -ForegroundColor Yellow
    
    # Versuche ohne Transaction
    sqlite3 $NEW_DB ".read export_current_data.sql" 2>&1
    sqlite3 $NEW_DB ".read export_exchanges_backup.sql" 2>&1
}

Write-Host "   ✓ Daten importiert" -ForegroundColor Green

# ====================================================================
# Verifizierung
# ====================================================================
Write-Host "`n[Verifizierung] Prüfe reparierte Datenbank..." -ForegroundColor Yellow

$tables = @("users", "ai_models", "traders", "user_signal_sources", "system_config", "beta_codes", "deals", "deal_events", "exchanges")

foreach ($table in $tables) {
    $count = sqlite3 $NEW_DB "SELECT COUNT(*) FROM $table;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ $table : $count Einträge" -ForegroundColor Green
    } else {
        Write-Host "❌ $table : Fehler beim Lesen" -ForegroundColor Red
    }
}

# Integritätsprüfung
Write-Host "`n   Führe Integritätsprüfung durch..." -ForegroundColor Gray
$integrity = sqlite3 $NEW_DB "PRAGMA integrity_check;" 2>&1
if ($integrity -eq "ok") {
    Write-Host "✅ Datenbank Integrität: OK" -ForegroundColor Green
} else {
    Write-Host "⚠ Integritätsprüfung: $integrity" -ForegroundColor Yellow
}

# ====================================================================
# Zusammenfassung
# ====================================================================
Write-Host "`n" + ("=" * 70) -ForegroundColor Gray
Write-Host "✅ REPARATUR ERFOLGREICH!" -ForegroundColor Green
Write-Host ("=" * 70) -ForegroundColor Gray

Write-Host "`nErstellte Dateien:" -ForegroundColor Cyan
Write-Host "  📁 $NEW_DB           - Reparierte Datenbank" -ForegroundColor White
Write-Host "  📁 $backupName       - Backup der Original-DB" -ForegroundColor White

Write-Host "`nNächste Schritte:" -ForegroundColor Cyan
Write-Host "  1. Prüfe die Daten in $NEW_DB" -ForegroundColor Yellow
Write-Host "  2. Ersetze die alte DB:" -ForegroundColor Yellow
Write-Host "     PS> Copy-Item $NEW_DB $CURRENT_DB -Force" -ForegroundColor Gray
Write-Host "  3. Falls Probleme auftreten:" -ForegroundColor Yellow
Write-Host "     PS> Copy-Item $backupName $CURRENT_DB -Force" -ForegroundColor Gray

Write-Host "`n⚠️  HINWEIS:" -ForegroundColor Yellow
Write-Host "   Die exchanges Daten stammen aus dem Backup und könnten älter sein!" -ForegroundColor Yellow

Write-Host "`n🎉 Fertig!" -ForegroundColor Green
