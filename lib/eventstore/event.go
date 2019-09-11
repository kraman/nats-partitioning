package eventstore

import (
	"time"
)

//EventData represents the event to be saved
type EventData struct {
	Data     []byte
	Metadata []byte
	Version  uint64
}

//RecordedEventData represents a saved event from the store
type RecordedEventData struct {
	EventData
	AggregateStream string
	AggregateID     string
	SeqNo           uint64
	Created         time.Time
}

type EventStore interface {
	AppendToStream(aggregateStream string, aggregateID string, events []EventData) error
	ReadEventStreamForAggregate(aggregateStream string, aggregateID string, startVersion uint64, follow bool, h func(*RecordedEventData, error))
	ReadEventStream(aggregateStream string, seqNo uint64, follow bool, h func(*RecordedEventData, error))
}
