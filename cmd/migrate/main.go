package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/database"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	migrationPath := filepath.Join("internal", "database", "migrations.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	if err := executeMigration(database.DB, string(sqlBytes)); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration completed successfully!")
}

func executeMigration(db *sql.DB, sql string) error {
	_, err := db.Exec(sql)
	return err
}
