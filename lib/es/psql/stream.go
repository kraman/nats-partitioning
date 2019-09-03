package psql

import (
	es "github.com/kraman/nats-test/lib/es"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

type psqlStream struct {
	conn *pgx.Conn
}

func New(conn *pgx.Conn) (es.EventStore, error) {
	_, err := conn.Exec(
		`CREATE TABLE IF NOT EXISTS event_store (
			aggregate_stream VARCHAR(50) NOT NULL,
			aggregate_id VARCHAR(50) NOT NULL,
			version BIGSERIAL NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			data BYTEA NOT NULL,
			metadata BYTEA NOT NULL,
			PRIMARY KEY(aggregate_stream, aggregate_id, version),
		)`,
	)
	if err != nil {
		return nil, errors.Wrap(err, "unable to determind if event_store table exists")
	}

	if _, err = conn.Prepare("append_event_stmt", `INSERT INTO event_stream (aggregate_stream, aggregate_id, data, metadata) VALUES ($1,$2,$3,$4)`); err != nil {
		return nil, errors.Wrap(err, "unable to prepare append statement")
	}

	if _, err = conn.Prepare("read_event_stmt", `SELECT * FROM event_stream WHERE aggregate_stream=$1 AND aggregate_id=$2 AND version > $3 LIMIT $4`); err != nil {
		return nil, errors.Wrap(err, "unable to prepare read statement")
	}

	return &psqlStream{
		conn: conn,
	}, nil
}

func (s *psqlStream) AppendToStream(aggregateStream string, aggregateID string, events []es.EventData) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "unable to begin transaction to append events")
	}
	for _, e := range events {
		_, err := tx.Exec("append_event_stmt", aggregateStream, aggregateID, e.Data, e.Metadata)
		if err != nil {
			return errors.Wrap(err, "unable to append events")
		}
	}
	return errors.Wrap(tx.Commit(), "unable to append events")
}

func (s *psqlStream) ReadEventStream(aggregateStream string, aggregateID string, startVersion es.EventVersion, follow bool, h func(*es.RecordedEventData, error)) {
	go func() {
		for {
			rows, err := s.conn.Query("read_event_stmt", aggregateStream, aggregateID, startVersion, 10)
			if err != nil {
				h(nil, err)
				return
			}
			nextVersion := startVersion
			for rows.Next() {
				ev := &es.RecordedEventData{}
				err := rows.Scan(&ev.AggregateStream, &ev.AggregateID, &ev.Version, &ev.Created, &ev.Data, &ev.Metadata)
				nextVersion = ev.Version
				h(ev, err)
			}
			if nextVersion == startVersion && !follow {
				break
			}
			startVersion = nextVersion
		}
	}()
}
