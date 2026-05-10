package cdr

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/safehouse/amigow/internal/config"
)

func Open(cfg config.CDRDatabase) (*sql.DB, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("cdr_db.host não configurado")
	}

	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=5s",
		cfg.Username, cfg.Password, cfg.Host, port, cfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão MySQL: %w", err)
	}

	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		log.Printf("[CDR] AVISO: banco MySQL inacessível no startup: %v", err)
	} else {
		log.Printf("[CDR] Conexão MySQL estabelecida com %s:%d/%s", cfg.Host, port, cfg.Database)
	}

	return db, nil
}

func SearchByLinkedID(ctx context.Context, db *sql.DB, table string, linkedid string) ([]map[string]any, error) {
	query := fmt.Sprintf(
		"SELECT * FROM `%s` WHERE linkedid = ? ORDER BY calldate ASC LIMIT 500",
		table,
	)

	rows, err := db.QueryContext(ctx, query, linkedid)
	if err != nil {
		return nil, fmt.Errorf("erro na query CDR: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter colunas: %w", err)
	}

	var result []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("erro ao fazer scan da linha: %w", err)
		}

		row := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar linhas: %w", err)
	}

	return result, nil
}
