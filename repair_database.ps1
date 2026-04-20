# ====================================================================
# Database Repair Script - Kombiniert aktuelle Daten mit Backup Exchanges Tabelle
# ====================================================================
# Problem: exchanges Tabelle in config.db ist korrupt (SQLITE_CORRUPT)
# Lösung: Exportiere funktionierende Tabellen + importiere exchanges aus Backup
# ====================================================================

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# Konfiguration
$CURRENT_DB = "config.db"
$BACKUP_DB = "config.db.corrupt_1762854593.db"
$NEW_DB = "config.db.repaired"
$TIMESTAMP = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "🔧 Database Repair Tool - Start" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Gray

# ====================================================================
# Schritt 1: Prüfe ob sqlite3.exe verfügbar ist
# ====================================================================
Write-Host "`n[1/6] Prüfe SQLite Installation..." -ForegroundColor Yellow

$sqliteCommand = $null
if (Get-Command sqlite3 -ErrorAction SilentlyContinue) {
    $sqliteCommand = "sqlite3"
    Write-Host "✅ SQLite3 gefunden: $(Get-Command sqlite3 | Select-Object -ExpandProperty Source)" -ForegroundColor Green
} else {
    Write-Host "❌ SQLite3 nicht gefunden!" -ForegroundColor Red
    Write-Host "   Bitte installiere SQLite3:" -ForegroundColor Yellow
    Write-Host "   - Download: https://www.sqlite.org/download.html" -ForegroundColor Yellow
    Write-Host "   - Oder: winget install SQLite.SQLite" -ForegroundColor Yellow
    exit 1
}

# ====================================================================
# Schritt 2: Backup der aktuellen Datenbank
# ====================================================================
Write-Host "`n[2/6] Erstelle Sicherheitskopie..." -ForegroundColor Yellow

if (-not (Test-Path $CURRENT_DB)) {
    Write-Host "❌ $CURRENT_DB nicht gefunden!" -ForegroundColor Red
    exit 1
}

$backupName = "config.db.backup_$TIMESTAMP"
Copy-Item $CURRENT_DB $backupName
Write-Host "✅ Backup erstellt: $backupName" -ForegroundColor Green

# ====================================================================
# Schritt 3: Prüfe Backup-Datenbank
# ====================================================================
Write-Host "`n[3/6] Prüfe Backup-Datenbank..." -ForegroundColor Yellow

if (-not (Test-Path $BACKUP_DB)) {
    Write-Host "❌ Backup-Datenbank nicht gefunden: $BACKUP_DB" -ForegroundColor Red
    exit 1
}

# Teste ob exchanges Tabelle im Backup lesbar ist
$testQuery = "SELECT COUNT(*) FROM exchanges;"
$result = & $sqliteCommand $BACKUP_DB $testQuery 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ exchanges Tabelle im Backup auch korrupt!" -ForegroundColor Red
    Write-Host "   Fehler: $result" -ForegroundColor Red
    exit 1
}
Write-Host "✅ Backup exchanges Tabelle ist lesbar ($result Einträge)" -ForegroundColor Green

# ====================================================================
# Schritt 4: Exportiere funktionierende Tabellen aus aktueller DB
# ====================================================================
Write-Host "`n[4/6] Exportiere aktuelle Daten (außer exchanges)..." -ForegroundColor Yellow

# Liste aller Tabellen (außer exchanges)
$tables = @(
    "users",
    "ai_models", 
    "traders",
    "user_signal_sources",
    "system_config",
    "beta_codes",
    "deals",
    "deal_events"
)

$exportFile = "export_current_data.sql"
$exportContent = @"
-- Exported from $CURRENT_DB at $TIMESTAMP
-- Excludes exchanges table (corrupted)

BEGIN TRANSACTION;

"@

foreach ($table in $tables) {
    Write-Host "   Exportiere: $table" -ForegroundColor Gray
    
    # Prüfe ob Tabelle existiert und lesbar ist
    $checkQuery = "SELECT COUNT(*) FROM $table;"
    $count = & $sqliteCommand $CURRENT_DB $checkQuery 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "      ✓ $count Einträge" -ForegroundColor Green
        
        # Exportiere Daten
        $dumpQuery = ".mode insert $table`nSELECT * FROM $table;"
        $tableData = & $sqliteCommand $CURRENT_DB $dumpQuery 2>&1
        
        if ($LASTEXITCODE -eq 0 -and $tableData) {
            $exportContent += "`n-- Table: $table ($count rows)`n"
            $exportContent += $tableData -join "`n"
            $exportContent += "`n"
        }
    } else {
        Write-Host "      ⚠ Tabelle nicht lesbar, wird übersprungen" -ForegroundColor Yellow
    }
}

$exportContent += "`nCOMMIT;`n"
$exportContent | Out-File -FilePath $exportFile -Encoding UTF8
Write-Host "✅ Export abgeschlossen: $exportFile" -ForegroundColor Green

# ====================================================================
# Schritt 5: Exportiere exchanges Tabelle aus Backup
# ====================================================================
Write-Host "`n[5/6] Exportiere exchanges Tabelle aus Backup..." -ForegroundColor Yellow

$exchangesExport = "export_exchanges_backup.sql"

# Erstelle SQL-Dump mit korrektem Format
$dumpScript = @"
.mode insert exchanges
.output $exchangesExport
SELECT * FROM exchanges;
.output stdout
"@

$dumpScriptFile = "dump_exchanges.sql"
$dumpScript | Out-File -FilePath $dumpScriptFile -Encoding ASCII

& $sqliteCommand $BACKUP_DB ".read $dumpScriptFile" 2>&1

if ($LASTEXITCODE -ne 0 -or -not (Test-Path $exchangesExport)) {
    Write-Host "❌ Fehler beim Export der exchanges Tabelle!" -ForegroundColor Red
    Write-Host "   Versuche alternative Methode..." -ForegroundColor Yellow
    
    # Alternative: Manueller Export per CSV und Konvertierung
    $csvExport = "exchanges_backup.csv"
    & $sqliteCommand $BACKUP_DB ".mode csv" ".headers on" ".output $csvExport" "SELECT * FROM exchanges;" ".output stdout" 2>&1
    
    if (Test-Path $csvExport) {
        Write-Host "   ✓ CSV Export erfolgreich" -ForegroundColor Green
        
        # Konvertiere CSV zu SQL INSERT Statements
        $csv = Import-Csv $csvExport
        $insertStatements = @()
        
        foreach ($row in $csv) {
            $values = @(
                "'$($row.id -replace "'", "''")'",
                "'$($row.user_id -replace "'", "''")'",
                "'$($row.name -replace "'", "''")'",
                "'$($row.type -replace "'", "''")'",
                "$($row.enabled)",
                "'$($row.api_key -replace "'", "''")'",
                "'$($row.secret_key -replace "'", "''")'",
                "$($row.testnet)",
                "'$($row.hyperliquid_wallet_addr -replace "'", "''")'",
                "'$($row.aster_user -replace "'", "''")'",
                "'$($row.aster_signer -replace "'", "''")'",
                "'$($row.aster_private_key -replace "'", "''")'",
                "'$($row.created_at)'",
                "'$($row.updated_at)'"
            )
            $insertStatements += "INSERT INTO exchanges VALUES($($values -join ', '));"
        }
        
        $exchangesContent = @"
-- Exported exchanges table from $BACKUP_DB
-- This data is from the backup and may be older!

BEGIN TRANSACTION;

$($insertStatements -join "`n")

COMMIT;
"@
        $exchangesContent | Out-File -FilePath $exchangesExport -Encoding UTF8
        Write-Host "✅ Exchanges Export abgeschlossen (via CSV): $exchangesExport" -ForegroundColor Green
    } else {
        Write-Host "❌ Auch CSV Export fehlgeschlagen!" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "✅ Exchanges Export abgeschlossen: $exchangesExport" -ForegroundColor Green
}

# ====================================================================
# Schritt 6: Erstelle neue Datenbank und importiere Daten
# ====================================================================
Write-Host "`n[6/6] Erstelle neue reparierte Datenbank..." -ForegroundColor Yellow

# Lösche alte reparierte DB falls vorhanden
if (Test-Path $NEW_DB) {
    Remove-Item $NEW_DB -Force
}

# Kopiere Schema von aktueller DB (nur Schema, keine Daten)
Write-Host "   Erstelle Schema..." -ForegroundColor Gray
$schemaQuery = ".schema"
$schema = & $sqliteCommand $CURRENT_DB $schemaQuery 2>&1

# Entferne problematische Zeilen aus Schema
$schema = $schema | Where-Object { 
    $_ -notmatch "PRAGMA" -and 
    $_ -notmatch "sqlite_sequence" -and
    $_ -notmatch "BEGIN TRANSACTION" -and
    $_ -notmatch "COMMIT" -and
    $_.Trim() -ne ""
}

$schemaFile = "schema.sql"
$schema -join "`n" | Out-File -FilePath $schemaFile -Encoding UTF8

# Erstelle neue DB mit Schema
& $sqliteCommand $NEW_DB ".read $schemaFile" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Fehler beim Erstellen des Schemas!" -ForegroundColor Red
    exit 1
}
Write-Host "   ✓ Schema erstellt" -ForegroundColor Green

# Importiere aktuelle Daten (ohne exchanges)
Write-Host "   Importiere aktuelle Daten..." -ForegroundColor Gray
& $sqliteCommand $NEW_DB ".read $exportFile" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Fehler beim Import der aktuellen Daten!" -ForegroundColor Red
    exit 1
}
Write-Host "   ✓ Aktuelle Daten importiert" -ForegroundColor Green

# Importiere exchanges aus Backup
Write-Host "   Importiere exchanges aus Backup..." -ForegroundColor Gray
& $sqliteCommand $NEW_DB ".read $exchangesExport" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Fehler beim Import der exchanges Tabelle!" -ForegroundColor Red
    exit 1
}
Write-Host "   ✓ Exchanges importiert" -ForegroundColor Green

# ====================================================================
# Schritt 7: Verifiziere neue Datenbank
# ====================================================================
Write-Host "`n[7/7] Verifiziere reparierte Datenbank..." -ForegroundColor Yellow

# Teste exchanges Tabelle
$verifyQuery = "SELECT COUNT(*) FROM exchanges;"
$verifyResult = & $sqliteCommand $NEW_DB $verifyQuery 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Verifikation fehlgeschlagen!" -ForegroundColor Red
    exit 1
}
Write-Host "✅ Exchanges Tabelle: $verifyResult Einträge" -ForegroundColor Green

# Teste andere Tabellen
foreach ($table in $tables) {
    $count = & $sqliteCommand $NEW_DB "SELECT COUNT(*) FROM $table;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ $table : $count Einträge" -ForegroundColor Green
    }
}

# Führe PRAGMA integrity_check durch
Write-Host "`n   Führe Integritätsprüfung durch..." -ForegroundColor Gray
$integrityCheck = & $sqliteCommand $NEW_DB "PRAGMA integrity_check;" 2>&1
if ($integrityCheck -eq "ok") {
    Write-Host "✅ Datenbank Integrität: OK" -ForegroundColor Green
} else {
    Write-Host "⚠ Integritätsprüfung: $integrityCheck" -ForegroundColor Yellow
}

# ====================================================================
# Zusammenfassung & nächste Schritte
# ====================================================================
Write-Host "`n" + ("=" * 70) -ForegroundColor Gray
Write-Host "✅ REPARATUR ERFOLGREICH!" -ForegroundColor Green
Write-Host ("=" * 70) -ForegroundColor Gray

Write-Host "`nErstellte Dateien:" -ForegroundColor Cyan
Write-Host "  📁 $NEW_DB           - Reparierte Datenbank" -ForegroundColor White
Write-Host "  📁 $backupName       - Backup der Original-DB" -ForegroundColor White
Write-Host "  📁 $exportFile       - Export der aktuellen Daten" -ForegroundColor White
Write-Host "  📁 $exchangesExport  - Export der exchanges Tabelle" -ForegroundColor White

Write-Host "`nNächste Schritte:" -ForegroundColor Cyan
Write-Host "  1. Prüfe die Daten in $NEW_DB" -ForegroundColor Yellow
Write-Host "  2. Teste die Anwendung mit der neuen DB:" -ForegroundColor Yellow
Write-Host "     > Copy-Item $NEW_DB $CURRENT_DB -Force" -ForegroundColor Gray
Write-Host "  3. Falls Probleme auftreten, kannst du zurück zum Backup:" -ForegroundColor Yellow
Write-Host "     > Copy-Item $backupName $CURRENT_DB -Force" -ForegroundColor Gray

Write-Host "`n⚠️  WICHTIG:" -ForegroundColor Yellow
Write-Host "   Die exchanges Daten sind aus dem Backup ($BACKUP_DB)" -ForegroundColor Yellow
Write-Host "   und könnten älter sein. Bitte prüfe die Konfigurationen!" -ForegroundColor Yellow

Write-Host "`n🎉 Fertig!" -ForegroundColor Green
