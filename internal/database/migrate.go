package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

const migrationAdvisoryLockID int64 = 740271904213

func RunMigrations() error {
	if DB == nil {
		return fmt.Errorf("database is not connected")
	}

	migrationPath := filepath.Join("internal", "database", "migrations.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	return executeMigration(DB, string(sqlBytes))
}

func executeMigration(db *sql.DB, migrationSQL string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT pg_advisory_lock($1)`, migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer tx.Exec(`SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockID)

	if _, err := tx.Exec(migrationSQL); err != nil {
		return err
	}

	return tx.Commit()
}
