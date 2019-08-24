package partitions

import (
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/kraman/nats-test/lib/es"
	stan "github.com/nats-io/stan.go"
)

func (po *PartitionOwners) Replay(ch <-chan *stan.Msg, sub stan.Subscription) error {
	t := time.NewTimer(time.Second)
	for {
		select {
		case <-t.C:
			sub.Close()
			return nil
		case m := <-ch:
			t.Reset(time.Second)
			if _, err := po.Apply(m); err != nil {
				return err
			}
		}
	}
}

func (po *PartitionOwners) Apply(m *stan.Msg) (evtAny *ptypes.DynamicAny, err error) {
	evtAny, err = es.UnpackEvent(m)
	if err != nil {
		return nil, err
	}

	switch e := evtAny.Message.(type) {
	case *AssignPartitionsEvent:
		po.AssignedOwners = e.Owners
	case *ReservePartitionEvent:
		po.ActualOwners[e.PartitionId] = e.OwnerId
	case *ReleasePartitionEvent:
		delete(po.ActualOwners, e.PartitionId)
	}

	return evtAny, nil
}
