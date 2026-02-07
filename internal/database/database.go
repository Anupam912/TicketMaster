package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	"event-ticketing-system/internal/config"

	_ "github.com/lib/pq"
)

var (
	DB *sql.DB
	ReadDB *sql.DB
	readDBMutex sync.RWMutex
)

func Connect(cfg *config.Config) error {
	var err error
	
	DB, err = sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * 60)

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Primary database connected successfully")

	if cfg.Database.ReadDSN() != "" {
		ReadDB, err = sql.Open("postgres", cfg.Database.ReadDSN())
		if err != nil {
			log.Printf("Warning: Failed to connect to read replica: %v. Using primary for reads.", err)
			ReadDB = nil
		} else {
			ReadDB.SetMaxOpenConns(25)
			ReadDB.SetMaxIdleConns(5)
			ReadDB.SetConnMaxLifetime(5 * 60)
			
			if err = ReadDB.Ping(); err != nil {
				log.Printf("Warning: Failed to ping read replica: %v. Using primary for reads.", err)
				ReadDB = nil
			} else {
				log.Println("Read replica connected successfully")
			}
		}
	}

	return nil
}

// GetReadDB returns the read replica if available, otherwise returns the primary DB
func GetReadDB() *sql.DB {
	readDBMutex.RLock()
	defer readDBMutex.RUnlock()
	
	if ReadDB != nil {
		return ReadDB
	}
	return DB
}

func Close() error {
	var err error
	if DB != nil {
		if closeErr := DB.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if ReadDB != nil {
		if closeErr := ReadDB.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; %v", err, closeErr)
			} else {
				err = closeErr
			}
		}
	}
	return err
}
