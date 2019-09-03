package es

import (
	"time"
)

//EventVersion is the version of an event being inserted or queried from the event store
type EventVersion uint64

//EventData represents the event to be saved
type EventData struct {
	Data     []byte
	Metadata []byte
}

//RecordedEventData represents a saved event from the store
type RecordedEventData struct {
	EventData
	AggregateStream string
	AggregateID     string
	Version         EventVersion
	Created         time.Time
}

type EventStore interface {
	AppendToStream(aggregateStream string, aggregateID string, events []EventData) error
	ReadEventStream(aggregateStream string, aggregateID string, startEventNumber EventVersion, follow bool, h func(*RecordedEventData, error))
}
