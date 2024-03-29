package nats

import (
	"fmt"
	"time"

	"github.com/kraman/nats-test/lib/cluster"

	"github.com/hashicorp/serf/serf"
)

type memberState struct {
	cluster.Member
	WallTime time.Time
	LTime    serf.LamportTime
}

func (ms *memberState) String() string {
	return fmt.Sprintf("%s, Wall: %s, Lamport: %v", ms.Member, ms.WallTime.String(), ms.LTime)
}
