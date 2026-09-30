package platform

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq" 
)

var DB *sql.DB

func InitDB() error {
	dsn := buildDSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("platform: sql.Open: %w", err)
	}

	
	if err := db.Ping(); err != nil {
		return fmt.Errorf("platform: db.Ping: %w — is Postgres running? (docker compose up -d)", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	DB = db
	log.Println("✅ Postgres connected")
	return nil
}


func buildDSN() string {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "notmysim")
	pass := getEnv("POSTGRES_PASSWORD", "notmysim_secret")
	name := getEnv("POSTGRES_DB", "notmysim")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, name,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
