package freepbx

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
)

type QueueMeta struct {
	Extension     string
	Descr         string
	Strategy      string
	Timeout       string
	Wrapuptime    string
	ServiceLevel  int
	MaxWait       string
	Ringing       int
	MonitorType   string
	QueueWait     int
	StaticMembers []string
}

func LoadQueues(ctx context.Context, db *sql.DB) ([]QueueMeta, error) {
	if db == nil {
		return nil, fmt.Errorf("freepbx: db handle nulo")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT extension, descr, ringing, maxwait, queuewait, monitor_type
		FROM queues_config
		ORDER BY extension`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar queues_config: %w", err)
	}
	defer rows.Close()

	var queues []QueueMeta
	for rows.Next() {
		var (
			extension   sql.NullString
			descr       sql.NullString
			ringing     sql.NullInt64
			maxwait     sql.NullString
			queuewait   sql.NullInt64
			monitorType sql.NullString
		)
		if err := rows.Scan(&extension, &descr, &ringing, &maxwait, &queuewait, &monitorType); err != nil {
			return nil, fmt.Errorf("erro ao ler queues_config: %w", err)
		}
		if !extension.Valid || extension.String == "" {
			continue
		}
		queues = append(queues, QueueMeta{
			Extension:   extension.String,
			Descr:       descr.String,
			Ringing:     int(ringing.Int64),
			MaxWait:     maxwait.String,
			QueueWait:   int(queuewait.Int64),
			MonitorType: monitorType.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar queues_config: %w", err)
	}

	for i := range queues {
		if err := fillDetails(ctx, db, &queues[i]); err != nil {
			log.Printf("[FREEPBX] erro ao carregar detalhes da queue %s: %v", queues[i].Extension, err)
		}
	}

	membersByQueue, err := loadSangomaMembers(ctx, db)
	if err != nil {
		log.Printf("[FREEPBX] erro ao carregar sangomartapi_call_queue_members: %v", err)
	} else {
		for i := range queues {
			queues[i].StaticMembers = membersByQueue[queues[i].Extension]
		}
	}

	return queues, nil
}

// loadSangomaMembers lê a lista de interfaces configuradas por queue em
// sangomartapi_call_queue_members. Usa apenas as colunas `interface` e
// `call_queue_account_id`; as demais não são confiáveis.
func loadSangomaMembers(ctx context.Context, db *sql.DB) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT interface, call_queue_account_id
		FROM sangomartapi_call_queue_members
		ORDER BY call_queue_account_id, interface`)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar sangomartapi_call_queue_members: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var iface, queueID sql.NullString
		if err := rows.Scan(&iface, &queueID); err != nil {
			return nil, fmt.Errorf("erro ao ler sangomartapi_call_queue_members: %w", err)
		}
		if !iface.Valid || iface.String == "" || !queueID.Valid || queueID.String == "" {
			continue
		}
		result[queueID.String] = append(result[queueID.String], iface.String)
	}
	return result, rows.Err()
}

func fillDetails(ctx context.Context, db *sql.DB, q *QueueMeta) error {
	rows, err := db.QueryContext(ctx, `
		SELECT keyword, data, flags
		FROM queues_details
		WHERE id = ?
		ORDER BY keyword, flags`, q.Extension)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			keyword sql.NullString
			data    sql.NullString
			flags   sql.NullInt64
		)
		if err := rows.Scan(&keyword, &data, &flags); err != nil {
			return err
		}
		switch keyword.String {
		case "strategy":
			q.Strategy = data.String
		case "timeout":
			q.Timeout = data.String
		case "wrapuptime":
			q.Wrapuptime = data.String
		case "servicelevel":
			if data.String != "" {
				if n, err := strconv.Atoi(data.String); err == nil {
					q.ServiceLevel = n
				}
			}
		}
	}
	return rows.Err()
}
