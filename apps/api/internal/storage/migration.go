package storage

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Migration represents a database migration
type Migration struct {
	Version     int       `json:"version"`
	Name        string    `json:"name"`
	UpSQL       string    `json:"up_sql"`
	DownSQL     string    `json:"down_sql"`
	Checksum    string    `json:"checksum"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	Description string    `json:"description"`
}

// MigrationManager handles database schema versioning
type MigrationManager struct {
	db            *sql.DB
	migrationsDir string
	tableName     string
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB, migrationsDir string) *MigrationManager {
	return &MigrationManager{
		db:            db,
		migrationsDir: migrationsDir,
		tableName:     "schema_migrations",
	}
}

// Initialize creates the migrations tracking table
func (m *MigrationManager) Initialize() error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		)
	`, m.tableName)

	_, err := m.db.Exec(createTableSQL)
	return err
}

// LoadMigrations loads all migration files from the migrations directory
func (m *MigrationManager) LoadMigrations() ([]Migration, error) {
	files, err := filepath.Glob(filepath.Join(m.migrationsDir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob migration files: %w", err)
	}

	var migrations []Migration
	for _, file := range files {
		migration, err := m.parseMigrationFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse migration file %s: %w", file, err)
		}
		migrations = append(migrations, migration)
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseMigrationFile parses a migration file and extracts up/down SQL
func (m *MigrationManager) parseMigrationFile(filePath string) (Migration, error) {
	filename := filepath.Base(filePath)
	
	// Parse version from filename (format: 001_migration_name.sql)
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return Migration{}, fmt.Errorf("invalid migration filename format: %s", filename)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return Migration{}, fmt.Errorf("invalid version in filename: %s", filename)
	}

	name := strings.TrimSuffix(parts[1], ".sql")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return Migration{}, fmt.Errorf("failed to read migration file: %w", err)
	}

	// Calculate checksum
	checksum := m.calculateChecksum(content)

	// Parse up and down SQL sections
	upSQL, downSQL, description := m.parseMigrationContent(string(content))

	return Migration{
		Version:     version,
		Name:        name,
		UpSQL:       upSQL,
		DownSQL:     downSQL,
		Checksum:    checksum,
		Description: description,
	}, nil
}

// parseMigrationContent parses migration file content for up/down SQL and description
func (m *MigrationManager) parseMigrationContent(content string) (upSQL, downSQL, description string) {
	lines := strings.Split(content, "\n")
	var currentSection string
	var upLines, downLines, descLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		switch {
		case strings.HasPrefix(trimmed, "-- +migrate Up"):
			currentSection = "up"
			continue
		case strings.HasPrefix(trimmed, "-- +migrate Down"):
			currentSection = "down"
			continue
		case strings.HasPrefix(trimmed, "-- Description:"):
			description = strings.TrimPrefix(trimmed, "-- Description:")
			description = strings.TrimSpace(description)
			continue
		case strings.HasPrefix(trimmed, "--"):
			if currentSection == "" {
				descLines = append(descLines, strings.TrimPrefix(trimmed, "--"))
			}
			continue
		}

		switch currentSection {
		case "up":
			upLines = append(upLines, line)
		case "down":
			downLines = append(downLines, line)
		}
	}

	if description == "" && len(descLines) > 0 {
		description = strings.TrimSpace(strings.Join(descLines, " "))
	}

	return strings.TrimSpace(strings.Join(upLines, "\n")),
		   strings.TrimSpace(strings.Join(downLines, "\n")),
		   description
}

// calculateChecksum calculates SHA256 checksum of migration content
func (m *MigrationManager) calculateChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}

// GetAppliedMigrations returns all applied migrations
func (m *MigrationManager) GetAppliedMigrations() ([]Migration, error) {
	query := fmt.Sprintf(`
		SELECT version, name, checksum, applied_at, description
		FROM %s
		ORDER BY version
	`, m.tableName)

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var migration Migration
		var appliedAt string
		
		err := rows.Scan(
			&migration.Version,
			&migration.Name,
			&migration.Checksum,
			&appliedAt,
			&migration.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", err)
		}

		migration.AppliedAt, _ = time.Parse("2006-01-02 15:04:05", appliedAt)
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// GetCurrentVersion returns the current schema version
func (m *MigrationManager) GetCurrentVersion() (int, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(version), 0)
		FROM %s
	`, m.tableName)

	var version int
	err := m.db.QueryRow(query).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// Migrate applies all pending migrations
func (m *MigrationManager) Migrate() error {
	allMigrations, err := m.LoadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create map of applied migrations for quick lookup
	appliedMap := make(map[int]Migration)
	for _, migration := range appliedMigrations {
		appliedMap[migration.Version] = migration
	}

	// Apply pending migrations
	for _, migration := range allMigrations {
		if applied, exists := appliedMap[migration.Version]; exists {
			// Verify checksum
			if applied.Checksum != migration.Checksum {
				return fmt.Errorf("checksum mismatch for migration %d: expected %s, got %s",
					migration.Version, applied.Checksum, migration.Checksum)
			}
			continue // Migration already applied
		}

		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// applyMigration applies a single migration
func (m *MigrationManager) applyMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration SQL
	if migration.UpSQL != "" {
		_, err = tx.Exec(migration.UpSQL)
		if err != nil {
			return fmt.Errorf("failed to execute migration SQL: %w", err)
		}
	}

	// Record migration in tracking table
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (version, name, checksum, description)
		VALUES (?, ?, ?, ?)
	`, m.tableName)

	_, err = tx.Exec(insertSQL, migration.Version, migration.Name, migration.Checksum, migration.Description)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// Rollback rolls back to a specific version
func (m *MigrationManager) Rollback(targetVersion int) error {
	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version %d must be less than current version %d",
			targetVersion, currentVersion)
	}

	allMigrations, err := m.LoadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Create map for quick lookup
	migrationMap := make(map[int]Migration)
	for _, migration := range allMigrations {
		migrationMap[migration.Version] = migration
	}

	// Rollback migrations in reverse order
	for version := currentVersion; version > targetVersion; version-- {
		migration, exists := migrationMap[version]
		if !exists {
			return fmt.Errorf("migration %d not found", version)
		}

		if err := m.rollbackMigration(migration); err != nil {
			return fmt.Errorf("failed to rollback migration %d: %w", version, err)
		}
	}

	return nil
}

// rollbackMigration rolls back a single migration
func (m *MigrationManager) rollbackMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute rollback SQL
	if migration.DownSQL != "" {
		_, err = tx.Exec(migration.DownSQL)
		if err != nil {
			return fmt.Errorf("failed to execute rollback SQL: %w", err)
		}
	}

	// Remove migration record
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE version = ?`, m.tableName)
	_, err = tx.Exec(deleteSQL, migration.Version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// ValidateIntegrity validates migration integrity
func (m *MigrationManager) ValidateIntegrity() error {
	allMigrations, err := m.LoadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create map of file migrations for quick lookup
	fileMap := make(map[int]Migration)
	for _, migration := range allMigrations {
		fileMap[migration.Version] = migration
	}

	// Validate applied migrations
	for _, applied := range appliedMigrations {
		file, exists := fileMap[applied.Version]
		if !exists {
			return fmt.Errorf("applied migration %d not found in migration files", applied.Version)
		}

		if applied.Checksum != file.Checksum {
			return fmt.Errorf("checksum mismatch for migration %d", applied.Version)
		}
	}

	return nil
}

// Status returns migration status information
type Status struct {
	CurrentVersion    int         `json:"current_version"`
	PendingMigrations []Migration `json:"pending_migrations"`
	AppliedCount      int         `json:"applied_count"`
	TotalCount        int         `json:"total_count"`
}

// Status returns current migration status
func (m *MigrationManager) Status() (*Status, error) {
	allMigrations, err := m.LoadMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Create map of applied migrations
	appliedMap := make(map[int]bool)
	for _, migration := range appliedMigrations {
		appliedMap[migration.Version] = true
	}

	// Find pending migrations
	var pendingMigrations []Migration
	for _, migration := range allMigrations {
		if !appliedMap[migration.Version] {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	return &Status{
		CurrentVersion:    currentVersion,
		PendingMigrations: pendingMigrations,
		AppliedCount:      len(appliedMigrations),
		TotalCount:        len(allMigrations),
	}, nil
}