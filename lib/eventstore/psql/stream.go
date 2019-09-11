package psql

import (
	es "github.com/kraman/nats-test/lib/eventstore"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

type psqlStream struct {
	conn *pgx.Conn
}

func New(conn *pgx.Conn) (es.EventStore, error) {
	_, err := conn.Exec(
		`CREATE TABLE IF NOT EXISTS event_store (
			id BIGSERIAL NOT NULL,
			aggregate_stream VARCHAR(50) NOT NULL,
			aggregate_id VARCHAR(50) NOT NULL,
			version NUMERIC NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			data BYTEA NOT NULL,
			metadata BYTEA NOT NULL,
			PRIMARY KEY(aggregate_stream, aggregate_id, version)
		);`,
	)
	if err != nil {
		return nil, errors.Wrap(err, "unable to determind if event_store table exists")
	}

	if _, err = conn.Prepare("append_event_stmt", `INSERT INTO event_store (aggregate_stream, aggregate_id, version, data, metadata) VALUES ($1, $2, $3, $4, $5);`); err != nil {
		return nil, errors.Wrap(err, "unable to prepare append statement")
	}

	if _, err = conn.Prepare("read_agg_event_stmt", `SELECT * FROM event_store WHERE aggregate_stream=$1 AND aggregate_id=$2 AND version >= $3 ORDER BY version LIMIT $4;`); err != nil {
		return nil, errors.Wrap(err, "unable to prepare read statement")
	}
	if _, err = conn.Prepare("read_event_stmt", `SELECT * FROM event_store WHERE aggregate_stream=$1 AND id >= $2 ORDER BY version LIMIT $3;`); err != nil {
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
		_, err := tx.Exec("append_event_stmt", aggregateStream, aggregateID, e.Version, e.Data, e.Metadata)
		if err != nil {
			return errors.Wrap(err, "unable to append events")
		}
	}
	return errors.Wrap(tx.Commit(), "unable to append events")
}

func (s *psqlStream) ReadEventStreamForAggregate(aggregateStream string, aggregateID string, startVersion uint64, follow bool, h func(*es.RecordedEventData, error)) {
	done := make(chan int, 1)
	go func() {
		for {
			rows, err := s.conn.Query("read_agg_event_stmt", aggregateStream, aggregateID, startVersion, 10)
			if err != nil {
				h(nil, err)
				done <- 1
				return
			}
			nextVersion := startVersion
			for rows.Next() {
				ev := &es.RecordedEventData{}
				err := rows.Scan(&ev.SeqNo, &ev.AggregateStream, &ev.AggregateID, &ev.Version, &ev.Created, &ev.Data, &ev.Metadata)
				nextVersion = ev.Version
				h(ev, err)
			}
			if nextVersion == startVersion && !follow {
				done <- 1
				break
			}
			startVersion = nextVersion + 1
		}
	}()
	<-done
}

func (s *psqlStream) ReadEventStream(aggregateStream string, seqNo uint64, follow bool, h func(*es.RecordedEventData, error)) {
	done := make(chan int, 1)
	go func() {
		for {
			rows, err := s.conn.Query("read_event_stmt", aggregateStream, seqNo, 10)
			if err != nil {
				h(nil, err)
				done <- 1
				return
			}
			nextSeqNo := seqNo
			for rows.Next() {
				ev := &es.RecordedEventData{}
				err := rows.Scan(&ev.SeqNo, &ev.AggregateStream, &ev.AggregateID, &ev.Version, &ev.Created, &ev.Data, &ev.Metadata)
				nextSeqNo = ev.SeqNo
				h(ev, err)
			}
			if nextSeqNo == seqNo && !follow {
				done <- 1
				break
			}
			seqNo = nextSeqNo + 1
		}
	}()
	<-done
}
