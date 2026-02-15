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
