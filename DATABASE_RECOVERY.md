# Database Recovery Summary

## Issue
SQLite database corruption: `sqlite3.DatabaseError: database disk image is malformed`

## Resolution
**Date:** November 11, 2025  
**Status:** ✅ RESOLVED

### Steps Taken
1. ✅ Stopped all Docker containers
2. ✅ Backed up corrupted database files
3. ✅ Rebuilt database from `dump_fixed.sql` (2.9 MB)
4. ✅ Verified database integrity (PASSED)
5. ✅ Restarted all services successfully

### Result
- All containers running properly
- sqlite-web accessible at http://localhost:8081
- Main application at http://localhost:8080
- Frontend at http://localhost:3000

## Prevention Strategies

### 1. Regular Automated Backups
Create a backup script that runs periodically:

```powershell
# backup_db.ps1
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$backupDir = "backups"

if (!(Test-Path $backupDir)) {
    New-Item -ItemType Directory -Path $backupDir
}

# Stop writes (optional - depends on your needs)
# docker compose exec nofx pkill -USR1 nofx

# Create backup
Copy-Item "config.db" "$backupDir/config.db.$timestamp"

# Keep only last 10 backups
Get-ChildItem "$backupDir/config.db.*" | 
    Sort-Object LastWriteTime -Descending | 
    Select-Object -Skip 10 | 
    Remove-Item

Write-Host "Backup created: $backupDir/config.db.$timestamp"
```

Schedule this with Windows Task Scheduler to run every 6-12 hours.

### 2. Use WAL Mode (Write-Ahead Logging)
WAL mode is more resilient to corruption. Add to your Go code:

```go
db.Exec("PRAGMA journal_mode=WAL;")
db.Exec("PRAGMA synchronous=NORMAL;")
```

### 3. Graceful Shutdown
Always stop containers gracefully:
```powershell
docker compose down  # Good
# Instead of: docker compose kill  # Bad
```

### 4. Database Health Checks
Add periodic integrity checks:

```powershell
# check_db.ps1
$result = docker run --rm -v "${PWD}:/data" keinos/sqlite3 sqlite3 /data/config.db "PRAGMA integrity_check;"
if ($result -ne "ok") {
    Write-Host "DATABASE INTEGRITY ISSUE!" -ForegroundColor Red
    # Send alert, create backup, etc.
}
```

### 5. Export to SQL Regularly
Keep fresh SQL dumps:

```powershell
# dump_db.ps1
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
docker run --rm -v "${PWD}:/data" keinos/sqlite3 sqlite3 /data/config.db ".dump" | 
    Out-File "dumps/dump.$timestamp.sql" -Encoding UTF8
```

### 6. Monitor Disk Space
Ensure adequate free space on the drive where config.db is stored. SQLite needs temporary space for operations.

### 7. Avoid Forced Shutdowns
- Don't force-quit Docker
- Don't power off the system without proper shutdown
- Use UPS if power instability is an issue

## Recovery Scripts

### Quick Recovery (if backup_ok.db is good)
```powershell
.\recover_database.ps1
```

### Full Rebuild (from SQL dump)
```powershell
.\rebuild_database.ps1
```

## Backup Files Location
- Good backups: `backups/` directory (create if doesn't exist)
- Corrupted files: `config.db.corrupted_*`, `config.db.bad_*`, etc.
- SQL dumps: `dump_fixed.sql`, `dump2.sql`, etc.

## Notes
- The corruption likely occurred due to unclean shutdown or disk issues
- WAL files (`.db-wal`, `.db-shm`) should be removed when restoring from backup
- Always verify integrity after recovery using `PRAGMA integrity_check;`
