package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// CallLog represents a single proxied request record.
type CallLog struct {
	ID             int64     `json:"id"`
	Route          string    `json:"route"`
	Method         string    `json:"method"`
	RequestURL     string    `json:"request_url"`
	RequestBody    string    `json:"request_body"`
	ResponseStatus int       `json:"response_status"`
	ResponseBody   string    `json:"response_body"`
	DurationMs     int64     `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

// Store wraps the database connection pool.
type Store struct {
	db *sql.DB
}

// New opens a connection pool and verifies connectivity.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return &Store{db: db}, nil
}

// InitSchema creates the call_logs table if it doesn't exist.
func (s *Store) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS call_logs (
		id              BIGINT AUTO_INCREMENT PRIMARY KEY,
		route           VARCHAR(255) NOT NULL,
		method          VARCHAR(10)  NOT NULL,
		request_url     TEXT         NOT NULL,
		request_body    LONGTEXT,
		response_status INT,
		response_body   LONGTEXT,
		duration_ms     INT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_created_at (created_at),
		INDEX idx_route (route)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	_, err := s.db.Exec(query)
	return err
}

// InsertLog saves a call log record.
func (s *Store) InsertLog(log *CallLog) error {
	query := `
	INSERT INTO call_logs (route, method, request_url, request_body, response_status, response_body, duration_ms)
	VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := s.db.Exec(query,
		log.Route, log.Method, log.RequestURL, log.RequestBody,
		log.ResponseStatus, log.ResponseBody, log.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("inserting log: %w", err)
	}
	id, _ := result.LastInsertId()
	log.ID = id
	return nil
}

// LogsResult holds a page of log results.
type LogsResult struct {
	Logs     []CallLog `json:"logs"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

// QueryLogs fetches call logs with optional route filtering and pagination.
func (s *Store) QueryLogs(route string, page, pageSize int) (*LogsResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	// Count total
	var countQuery string
	var countArgs []any
	if route != "" {
		countQuery = "SELECT COUNT(*) FROM call_logs WHERE route = ?"
		countArgs = []any{route}
	} else {
		countQuery = "SELECT COUNT(*) FROM call_logs"
	}

	var total int64
	if err := s.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting logs: %w", err)
	}

	// Query page
	var selectQuery string
	var selectArgs []any
	if route != "" {
		selectQuery = `
		SELECT id, route, method, request_url,
		       COALESCE(response_status, 0),
		       COALESCE(duration_ms, 0),
		       created_at
		FROM call_logs
		WHERE route = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?`
		selectArgs = []any{route, pageSize, offset}
	} else {
		selectQuery = `
		SELECT id, route, method, request_url,
		       COALESCE(response_status, 0),
		       COALESCE(duration_ms, 0),
		       created_at
		FROM call_logs
		ORDER BY id DESC
		LIMIT ? OFFSET ?`
		selectArgs = []any{pageSize, offset}
	}

	rows, err := s.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying logs: %w", err)
	}
	defer rows.Close()

	var logs []CallLog
	for rows.Next() {
		var l CallLog
		if err := rows.Scan(&l.ID, &l.Route, &l.Method, &l.RequestURL,
			&l.ResponseStatus, &l.DurationMs, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning log row: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating log rows: %w", err)
	}

	if logs == nil {
		logs = []CallLog{}
	}

	return &LogsResult{
		Logs:     logs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetLogByID fetches a single log entry with full request and response bodies.
func (s *Store) GetLogByID(id int64) (*CallLog, error) {
	var l CallLog
	query := `
	SELECT id, route, method, request_url,
	       COALESCE(request_body, ''),
	       COALESCE(response_status, 0),
	       COALESCE(response_body, ''),
	       COALESCE(duration_ms, 0),
	       created_at
	FROM call_logs
	WHERE id = ?`
	if err := s.db.QueryRow(query, id).Scan(
		&l.ID, &l.Route, &l.Method, &l.RequestURL,
		&l.RequestBody, &l.ResponseStatus, &l.ResponseBody,
		&l.DurationMs, &l.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("querying log by id: %w", err)
	}
	return &l, nil
}

// DistinctRoutes returns the set of route prefixes that have been logged.
func (s *Store) DistinctRoutes() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT route FROM call_logs ORDER BY route")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

// Close shuts down the connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}
