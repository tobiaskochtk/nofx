# Rebuild SQLite Database from SQL Dump
$ErrorActionPreference = "Stop"

Write-Host "=== Database Rebuild Script ===" -ForegroundColor Cyan
Write-Host ""

# Stop containers
Write-Host "[1/4] Stopping Docker containers..." -ForegroundColor Yellow
docker compose down
Start-Sleep -Seconds 2

# Backup current database
Write-Host "[2/4] Backing up current database..." -ForegroundColor Yellow
$timestamp = Get-Date -Format "yyyyMMddHHmmss"
if (Test-Path "config.db") {
    Move-Item "config.db" "config.db.old_$timestamp" -Force
}
if (Test-Path "config.db-shm") { Remove-Item "config.db-shm" -Force }
if (Test-Path "config.db-wal") { Remove-Item "config.db-wal" -Force }

# Rebuild from SQL dump using Docker
Write-Host "[3/4] Rebuilding database from dump_fixed.sql..." -ForegroundColor Yellow
Write-Host "   This may take a moment..." -ForegroundColor Gray

# Use Docker with sqlite3 to rebuild
Get-Content "dump_fixed.sql" | docker run --rm -i -v "${PWD}:/data" -w /data keinos/sqlite3 sqlite3 /data/config.db

if (Test-Path "config.db") {
    $size = (Get-Item "config.db").Length
    Write-Host "   New database created: config.db ($([math]::Round($size/1MB, 2)) MB)" -ForegroundColor Green
    
    # Verify integrity
    Write-Host "   Verifying integrity..." -ForegroundColor Gray
    $check = docker run --rm -v "${PWD}:/data" keinos/sqlite3 sqlite3 /data/config.db "PRAGMA integrity_check;"
    if ($check -eq "ok") {
        Write-Host "   Database integrity: OK" -ForegroundColor Green
    } else {
        Write-Host "   Database integrity: ISSUES FOUND" -ForegroundColor Red
        Write-Host "   $check" -ForegroundColor Red
    }
} else {
    Write-Host "   ERROR: Database creation failed!" -ForegroundColor Red
    exit 1
}

# Restart containers
Write-Host "[4/4] Starting Docker containers..." -ForegroundColor Yellow
docker compose up -d

Write-Host ""
Write-Host "=== Rebuild Complete ===" -ForegroundColor Green
Write-Host "Check the application at http://localhost:3000" -ForegroundColor Cyan
Write-Host "SQLite web interface should be available shortly" -ForegroundColor Cyan
Write-Host ""
