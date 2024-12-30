package sqlite_queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/DonovanKelly/go-wait-your-turn/sqlc"
	"sync"
)

const writeBufferSize = 100

type SqlcGeneric func(*sqlc.Queries, context.Context, any) error

type WriteTx struct {
	ErrChan chan error
	Query   SqlcGeneric
	Args    interface{}
}

type Queue struct {
	Queries    *sqlc.Queries
	Db         *sql.DB
	WriteQueue chan WriteTx
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewQueue(sqlDb *sql.DB, ctx context.Context) (*Queue, error) {
	ctx, cancel := context.WithCancel(context.Background())
	sqldb := &Queue{
		Queries:    sqlc.New(sqlDb),
		Db:         sqlDb,
		WriteQueue: make(chan WriteTx, writeBufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	return sqldb, nil
}

func (d *Queue) Start() {
	d.wg.Add(1)
	defer d.wg.Done()
	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			case writeTx := <-d.WriteQueue:
				err := writeTx.Query(d.Queries, d.ctx, writeTx.Args)
				writeTx.ErrChan <- err
			}
		}
	}()
}

func (d *Queue) Stop() error {
	d.cancel()
	d.wg.Wait()
	close(d.WriteQueue)
	return d.Db.Close()
}

func (d *Queue) EnqueueWriteTx(queryFunc SqlcGeneric, args any) error {
	select {
	case <-d.ctx.Done():
		return errors.New("database is shutting down")
	default:
	}

	errChan := make(chan error, 1)
	writeTx := WriteTx{
		Query:   queryFunc,
		Args:    args,
		ErrChan: errChan,
	}
	d.WriteQueue <- writeTx
	return <-errChan
}

func OpenSqliteDb(dbPath string) (*sql.DB, error) {
	sqliteDb, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := sqliteDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return sqliteDb, nil
}
