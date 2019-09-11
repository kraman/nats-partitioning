package es

import (
	proto "github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	stan "github.com/nats-io/stan.go"
	"github.com/pkg/errors"
)

func UnpackEvent(m *stan.Msg) (evtAny *ptypes.DynamicAny, err error) {
	evt := &EventMessage{}
	if err := evt.Unmarshal(m.Data); err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal message")
	}
	evtAny = &ptypes.DynamicAny{}
	if err := ptypes.UnmarshalAny(evt.Data, evtAny); err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal message event data")
	}
	return evtAny, nil
}

func PackEvent(evtData proto.Message) ([]byte, error) {
	evtAny, err := ptypes.MarshalAny(evtData)
	if err != nil {
		return nil, err
	}
	evt := EventMessage{
		Data:      evtAny,
		Timestamp: ptypes.TimestampNow(),
	}
	return evt.Marshal()
}
