# SQLite Database Recovery Script
# This script attempts to recover a corrupted SQLite database

$ErrorActionPreference = "Stop"
$timestamp = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "=== SQLite Database Recovery Script ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Stop Docker containers
Write-Host "[1/5] Stopping Docker containers..." -ForegroundColor Yellow
docker compose down
Start-Sleep -Seconds 2

# Step 2: Backup the corrupted database
Write-Host "[2/5] Backing up corrupted database..." -ForegroundColor Yellow
if (Test-Path "config.db") {
    Copy-Item "config.db" "config.db.corrupted_$timestamp"
    if (Test-Path "config.db-shm") { Copy-Item "config.db-shm" "config.db-shm.corrupted_$timestamp" }
    if (Test-Path "config.db-wal") { Copy-Item "config.db-wal" "config.db-wal.corrupted_$timestamp" }
    Write-Host "   Backed up to: config.db.corrupted_$timestamp" -ForegroundColor Green
}

# Step 3: Try to dump and recover data from corrupted database
Write-Host "[3/5] Attempting to recover data from corrupted database..." -ForegroundColor Yellow
$recoveryAttempt = $false
if (Test-Path "config.db") {
    try {
        # Try to dump the database
        sqlite3 config.db ".dump" | Out-File "recovered_dump_$timestamp.sql" -Encoding UTF8
        if ((Get-Item "recovered_dump_$timestamp.sql").Length -gt 0) {
            Write-Host "   Recovery dump created: recovered_dump_$timestamp.sql" -ForegroundColor Green
            $recoveryAttempt = $true
        }
    } catch {
        Write-Host "   Unable to dump corrupted database: $_" -ForegroundColor Red
    }
}

# Step 4: Restore from backup
Write-Host "[4/5] Restoring database from backup..." -ForegroundColor Yellow
if (Test-Path "backup_ok.db") {
    # Remove corrupted files
    Remove-Item "config.db" -Force -ErrorAction SilentlyContinue
    Remove-Item "config.db-shm" -Force -ErrorAction SilentlyContinue
    Remove-Item "config.db-wal" -Force -ErrorAction SilentlyContinue
    
    # Copy good backup
    Copy-Item "backup_ok.db" "config.db"
    Write-Host "   Database restored from backup_ok.db" -ForegroundColor Green
    
    # If we recovered data, offer to merge it
    if ($recoveryAttempt) {
        Write-Host ""
        Write-Host "   A recovery dump was created: recovered_dump_$timestamp.sql" -ForegroundColor Cyan
        Write-Host "   You may want to review it and manually merge any recent data." -ForegroundColor Cyan
    }
} else {
    Write-Host "   ERROR: backup_ok.db not found!" -ForegroundColor Red
    Write-Host "   Please locate a good backup before proceeding." -ForegroundColor Red
    exit 1
}

# Step 5: Verify and restart
Write-Host "[5/5] Verifying database integrity..." -ForegroundColor Yellow
try {
    $integrityCheck = sqlite3 config.db "PRAGMA integrity_check;" 2>&1
    if ($integrityCheck -like "*ok*") {
        Write-Host "   Database integrity check: PASSED" -ForegroundColor Green
    } else {
        Write-Host "   Database integrity check: FAILED" -ForegroundColor Red
        Write-Host "   Output: $integrityCheck" -ForegroundColor Red
    }
} catch {
    Write-Host "   Could not verify (sqlite3 not available): $_" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Recovery Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Review the recovery status above"
Write-Host "2. Start Docker containers: docker compose up -d"
Write-Host "3. Check the application logs"
Write-Host "4. Consider implementing regular backups to prevent data loss"
Write-Host ""
