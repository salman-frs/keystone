# Database Management Guide

## Overview

This guide covers database schema versioning, migration management, and maintenance procedures for the Keystone DevSecOps platform. The system uses SQLite with a custom migration framework for reliable schema evolution.

## Migration System

### Migration File Structure

Migration files follow a strict naming convention and structure:

```
apps/api/internal/storage/migrations/
├── 001_initial_schema.sql
├── 002_add_policy_evaluation.sql
└── 003_add_attestation_tracking.sql
```

**Naming Convention:**
- Format: `{version}_{description}.sql`
- Version: Zero-padded 3-digit number (001, 002, 003...)
- Description: Snake_case description of changes

**File Structure:**
```sql
-- Description: Brief description of the migration purpose

-- +migrate Up
CREATE TABLE example_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +migrate Down
DROP TABLE IF EXISTS example_table;
```

### Creating New Migrations

**1. Generate Migration File:**
```bash
# Create new migration file with next version number
./scripts/create-migration.sh "add_user_authentication"

# This creates: 003_add_user_authentication.sql
```

**2. Edit Migration Content:**
```sql
-- Description: Add user authentication and session management

-- +migrate Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    session_token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create indexes
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_sessions_token ON user_sessions(session_token);
CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_user_sessions_expires;
DROP INDEX IF EXISTS idx_user_sessions_token;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_username;

DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS users;
```

### Running Migrations

**Apply All Pending Migrations:**
```bash
# Run migration command
go run ./cmd/migrate/main.go up

# Output:
# Applying migration 001: initial_schema
# Applying migration 002: add_policy_evaluation
# Applying migration 003: add_user_authentication
# All migrations completed successfully
```

**Check Migration Status:**
```bash
# Check current status
go run ./cmd/migrate/main.go status

# Output:
# Current version: 3
# Applied migrations: 3
# Pending migrations: 0
# 
# Applied:
# ✅ 001: initial_schema (2025-09-21 10:30:00)
# ✅ 002: add_policy_evaluation (2025-09-21 10:31:15)
# ✅ 003: add_user_authentication (2025-09-21 10:32:30)
```

**Validate Migration Integrity:**
```bash
# Validate checksums and integrity
go run ./cmd/migrate/main.go validate

# Output:
# ✅ Migration 001: checksum valid
# ✅ Migration 002: checksum valid
# ✅ Migration 003: checksum valid
# All migrations validated successfully
```

### Rollback Procedures

**Rollback to Specific Version:**
```bash
# Rollback to version 2 (will undo migration 3)
go run ./cmd/migrate/main.go down 2

# Output:
# Rolling back migration 003: add_user_authentication
# Rollback completed. Current version: 2
```

**Rollback One Version:**
```bash
# Rollback one step
go run ./cmd/migrate/main.go down

# Equivalent to: go run ./cmd/migrate/main.go down $(current_version - 1)
```

**Emergency Rollback (Force):**
```bash
# Force rollback without validation (use with caution)
go run ./cmd/migrate/main.go down 1 --force

# This bypasses safety checks - only use for recovery
```

## Migration Best Practices

### Writing Safe Migrations

**1. Always Provide Down Migration:**
```sql
-- ❌ Bad: No down migration
-- +migrate Up
CREATE TABLE new_table (id INTEGER PRIMARY KEY);

-- ✅ Good: Complete up/down migrations
-- +migrate Up
CREATE TABLE new_table (id INTEGER PRIMARY KEY);

-- +migrate Down
DROP TABLE IF EXISTS new_table;
```

**2. Use IF EXISTS/IF NOT EXISTS:**
```sql
-- ✅ Safe table creation
CREATE TABLE IF NOT EXISTS user_preferences (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL
);

-- ✅ Safe index creation
CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id ON user_preferences(user_id);

-- ✅ Safe table removal
DROP TABLE IF EXISTS user_preferences;
```

**3. Add Indexes for Performance:**
```sql
-- Always create indexes for:
-- - Foreign key columns
-- - Frequently queried columns
-- - Columns used in WHERE clauses

CREATE INDEX idx_scan_results_repository ON scan_results(repository_owner, repository_name);
CREATE INDEX idx_vulnerability_cache_expires ON vulnerability_cache(cache_expires_at);
```

**4. Handle Large Data Changes Carefully:**
```sql
-- ✅ Good: Batch updates for large tables
-- +migrate Up
UPDATE vulnerability_cache 
SET severity = 'CRITICAL' 
WHERE cvss_score >= 9.0 
AND severity != 'CRITICAL';

-- Consider breaking into smaller batches for very large tables
```

### Testing Migrations

**1. Test on Development Data:**
```bash
# Create test database copy
cp production.db test_migration.db

# Test migration on copy
KEYSTONE_DB_PATH=test_migration.db go run ./cmd/migrate/main.go up

# Verify data integrity
sqlite3 test_migration.db "SELECT COUNT(*) FROM vulnerability_cache;"
```

**2. Test Rollback Procedures:**
```bash
# Apply migration
go run ./cmd/migrate/main.go up

# Test rollback
go run ./cmd/migrate/main.go down $(current_version - 1)

# Verify data is restored
sqlite3 keystone.db ".schema"
```

**3. Performance Testing:**
```bash
# Measure migration time
time go run ./cmd/migrate/main.go up

# Check database size impact
ls -lh keystone.db
```

## Database Maintenance

### Regular Maintenance Tasks

**Weekly Tasks:**
```bash
#!/bin/bash
# Script: weekly-db-maintenance.sh

# Vacuum database to reclaim space
sqlite3 keystone.db "VACUUM;"

# Analyze tables for query optimization
sqlite3 keystone.db "ANALYZE;"

# Check database integrity
sqlite3 keystone.db "PRAGMA integrity_check;"

# Clean expired cache entries
sqlite3 keystone.db "DELETE FROM vulnerability_cache WHERE cache_expires_at < datetime('now');"
```

**Monthly Tasks:**
```bash
#!/bin/bash
# Script: monthly-db-maintenance.sh

# Backup database
cp keystone.db "backups/keystone_$(date +%Y%m%d).db"

# Compress old backups
gzip backups/keystone_$(date -d '30 days ago' +%Y%m%d).db

# Remove backups older than 90 days
find backups/ -name "keystone_*.db.gz" -mtime +90 -delete

# Update database statistics
sqlite3 keystone.db "ANALYZE main;"
```

### Backup and Recovery

**Creating Backups:**
```bash
# Hot backup (database remains accessible)
sqlite3 keystone.db ".backup backup_$(date +%Y%m%d_%H%M%S).db"

# Cold backup (stop application first)
cp keystone.db "keystone_backup_$(date +%Y%m%d_%H%M%S).db"

# Compressed backup
sqlite3 keystone.db ".dump" | gzip > "keystone_$(date +%Y%m%d).sql.gz"
```

**Restoring from Backup:**
```bash
# Restore from .db backup
cp keystone_backup_20250921_103000.db keystone.db

# Restore from SQL dump
gunzip -c keystone_20250921.sql.gz | sqlite3 new_keystone.db

# Verify restored database
go run ./cmd/migrate/main.go validate
```

### Performance Optimization

**Query Performance:**
```bash
# Enable query planning
sqlite3 keystone.db "PRAGMA query_planner = ON;"

# Analyze slow queries
sqlite3 keystone.db "EXPLAIN QUERY PLAN SELECT * FROM vulnerability_cache WHERE cve_id = 'CVE-2024-1234';"

# Update table statistics
sqlite3 keystone.db "ANALYZE vulnerability_cache;"
```

**Storage Optimization:**
```bash
# Check table sizes
sqlite3 keystone.db "SELECT name, COUNT(*) as rows FROM sqlite_master m JOIN (SELECT name FROM sqlite_master WHERE type='table') t GROUP BY name;"

# Identify unused space
sqlite3 keystone.db "PRAGMA freelist_count;"

# Reclaim space
sqlite3 keystone.db "VACUUM;"
```

**Index Management:**
```bash
# List all indexes
sqlite3 keystone.db ".indexes"

# Analyze index usage
sqlite3 keystone.db "PRAGMA index_info(idx_vulnerability_cache_cve_id);"

# Drop unused indexes (carefully!)
sqlite3 keystone.db "DROP INDEX IF EXISTS unused_index_name;"
```

## Troubleshooting

### Common Issues

**Migration Checksum Mismatch:**
```bash
# Problem: Migration file was modified after being applied
# Error: checksum mismatch for migration 002: expected abc123, got def456

# Solution 1: Revert file changes to original state
git checkout HEAD -- apps/api/internal/storage/migrations/002_add_policy_evaluation.sql

# Solution 2: Create new migration to fix issues
./scripts/create-migration.sh "fix_policy_evaluation_schema"
```

**Database Locked Error:**
```bash
# Problem: Database is locked by another process
# Error: database is locked

# Solution 1: Check for running processes
ps aux | grep keystone

# Solution 2: Force unlock (use with caution)
fuser keystone.db  # Shows processes using the file
kill -9 <process_id>

# Solution 3: Check for .db-wal and .db-shm files
ls -la keystone.db*
rm keystone.db-wal keystone.db-shm  # Only if application is stopped
```

**Disk Space Issues:**
```bash
# Problem: Insufficient disk space during migration
# Error: disk I/O error

# Solution 1: Check available space
df -h .

# Solution 2: Clean up old data
sqlite3 keystone.db "DELETE FROM scan_results WHERE created_at < datetime('now', '-30 days');"
sqlite3 keystone.db "VACUUM;"

# Solution 3: Move database to larger partition
mv keystone.db /larger/partition/keystone.db
ln -s /larger/partition/keystone.db keystone.db
```

**Corrupt Database:**
```bash
# Check corruption
sqlite3 keystone.db "PRAGMA integrity_check;"

# Attempt repair
sqlite3 keystone.db ".dump" | sqlite3 keystone_repaired.db

# Restore from backup if repair fails
cp keystone_backup_latest.db keystone.db
```

### Recovery Procedures

**Complete Database Recovery:**
```bash
#!/bin/bash
# Script: emergency-db-recovery.sh

echo "Starting database recovery procedure..."

# 1. Stop application
systemctl stop keystone-api

# 2. Create emergency backup
cp keystone.db keystone_emergency_backup.db

# 3. Check integrity
if sqlite3 keystone.db "PRAGMA integrity_check;" | grep -q "ok"; then
    echo "Database integrity OK"
else
    echo "Database corruption detected, restoring from backup"
    cp keystone_backup_latest.db keystone.db
fi

# 4. Validate migrations
if go run ./cmd/migrate/main.go validate; then
    echo "Migration validation passed"
else
    echo "Migration validation failed, manual intervention required"
    exit 1
fi

# 5. Restart application
systemctl start keystone-api

echo "Database recovery completed"
```

## Schema Documentation

### Current Schema (Version 2)

**vulnerability_cache:**
- Stores cached vulnerability data from multiple sources
- TTL-based expiration for cache invalidation
- Supports NVD, GitHub, Trivy, and Grype data sources

**external_service_status:**
- Tracks external service availability and performance
- Monitors rate limits and circuit breaker states
- Used for service health dashboards

**scan_results:**
- Stores vulnerability scan results and metadata
- Links to vulnerability correlations
- Tracks scan status and timing

**vulnerability_correlations:**
- Correlates vulnerabilities across multiple scanners
- Calculates confidence scores for accuracy
- Provides final severity assessment

**policy_definitions:**
- Stores OPA/Rego policy definitions
- Supports versioning and activation controls
- Categorizes policies by type and severity

**policy_evaluations:**
- Records policy evaluation results
- Links evaluations to specific scans
- Tracks performance metrics

**compliance_reports:**
- Generates compliance framework reports
- Aggregates policy evaluation results
- Supports multiple compliance standards

### Data Retention Policies

**Short-term Data (7 days):**
- External service status checks
- Real-time rate limit information
- Temporary scan artifacts

**Medium-term Data (30 days):**
- Scan results and correlation data
- Policy evaluation results
- Performance metrics

**Long-term Data (1 year):**
- Vulnerability cache entries
- Compliance reports
- Migration history

**Permanent Data:**
- Policy definitions
- Schema migration records
- Configuration settings