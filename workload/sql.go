package workload

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/stdlib"
)

type SQL struct {
	DSN     string        `yaml:"sql_dsn"`
	Latency time.Duration `yaml:"sql_latency"`
	db      *sql.DB
}

func (s *SQL) Setup() error {
	if s.Latency == 0 {
		s.Latency = 10 * time.Millisecond
	}
	var err error
	s.db, err = sql.Open("pgx", s.DSN)
	if err != nil {
		return err
	}
	return s.db.Ping()
}

func (s *SQL) Run() error {
	q := `SELECT 1+1 AS calc FROM pg_sleep_for('10ms');`
	var answer int
	if err := s.db.QueryRow(q).Scan(&answer); err != nil {
		return err
	} else if answer != 2 {
		return fmt.Errorf("bad answer=%d want=%d", answer, 2)
	}
	return nil
}
