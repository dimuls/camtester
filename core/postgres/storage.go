package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/dimuls/camtester/core/entity"

	"github.com/Boostport/migration"
	"github.com/Boostport/migration/driver/postgres"
	"github.com/gobuffalo/packr"
	"github.com/jmoiron/sqlx"
)

type Storage struct {
	db  *sqlx.DB
	uri string
}

func NewStorage(uri string) (
	*Storage, error) {
	db, err := sqlx.Open("postgres", uri)
	if err != nil {
		return nil, errors.New("failed to open DB: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)

	err = db.PingContext(ctx)
	cancel()
	if err != nil {
		return nil, errors.New("failed to ping DB: " + err.Error())
	}

	return &Storage{
		db:  db,
		uri: uri,
	}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

//go:generate packr

const migrationsPath = "./migrations"

func (s *Storage) Migrate() error {
	packrSource := &migration.PackrMigrationSource{
		Box: packr.NewBox(migrationsPath),
	}

	d, err := postgres.New(s.uri)
	if err != nil {
		return errors.New("failed to create migration driver: " + err.Error())
	}

	_, err = migration.Migrate(d, packrSource, migration.Up, 0)
	if err != nil {
		return errors.New("failed to migrate: " + err.Error())
	}

	return nil
}

func (s *Storage) AddTask(t entity.Task) (id int, err error) {
	err = s.db.QueryRow(`
		insert into task (type, payload)
		values ($1, $2)
		returning id
	`, t.Type, t.Payload).Scan(&id)
	return
}

func (s *Storage) TaskResult(taskID int) (tr entity.TaskResult, err error) {
	err = s.db.QueryRowx(`
		select * from task_result where task_id = $1
	`, taskID).StructScan(&tr)
	return
}

func (s *Storage) AddTaskResult(tr entity.TaskResult) (err error) {
	_, err = s.db.Exec(`
		insert into task_result (task_id, ok, payload)
		values ($1, $2, $3)
	`, tr.TaskID, tr.Ok, tr.Payload)
	return
}
