package psql

import (
	"testing"

	"github.com/jackc/pgx"
	es "github.com/kraman/nats-test/lib/eventstore"
)

func Test_psqlStream_AppendToStream(t *testing.T) {
	conn, err := pgx.Connect(pgx.ConnConfig{
		Host:     "localhost",
		User:     "gotest",
		Password: "gotest",
		Database: "gotest",
	})
	if err != nil {
		t.Fatalf("unable to connect to postgres test database: %v", err)
		return
	}

	type fields struct {
		conn *pgx.Conn
	}
	type args struct {
		aggregateStream  string
		aggregateID      string
		events           []es.EventData
		startReadVersion uint64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool

		startReadVersion uint64
	}{
		{
			name:   "append 1 event",
			fields: fields{conn: conn},
			args: args{
				aggregateStream: "stream1",
				aggregateID:     "agg1",
				events: []es.EventData{
					es.EventData{Version: 0, Data: []byte{0}, Metadata: []byte("md1")},
				},
				startReadVersion: 0},
		},
		{
			name:   "append 2 events",
			fields: fields{conn: conn},
			args: args{
				aggregateStream: "stream1",
				aggregateID:     "agg2",
				events: []es.EventData{
					es.EventData{Version: 0, Data: []byte{0}, Metadata: []byte("md1")},
					es.EventData{Version: 1, Data: []byte{1}, Metadata: []byte("md2")},
				},
				startReadVersion: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.fields.conn)
			if (err != nil) != tt.wantErr {
				t.Errorf("psqlStream.New() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.fields.conn.Exec("DELETE FROM event_store;")

			if err := s.AppendToStream(tt.args.aggregateStream, tt.args.aggregateID, tt.args.events); (err != nil) != tt.wantErr {
				t.Errorf("psqlStream.AppendToStream() error = %v, wantErr %v", err, tt.wantErr)
			}

			i := uint64(0)
			s.ReadEventStreamForAggregate(tt.args.aggregateStream, tt.args.aggregateID, tt.args.startReadVersion, false, func(e *es.RecordedEventData, err error) {
				if (err != nil) != tt.wantErr {
					t.Errorf("psqlStream.ReadEventStream() error = %v, wantErr %v", err, tt.wantErr)
				}

				if string(e.Data) != string(tt.args.events[i].Data) {
					t.Logf("event data doesnt match. expected %v, got %v", tt.args.events[i].Data, e.Data)
				}

				if e.Version != i {
					t.Errorf("event data version doesnt match. expected %d, got %d", tt.args.events[i].Version, i)
				}
				i++
			})
		})
	}
}
