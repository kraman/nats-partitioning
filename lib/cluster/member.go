package cluster

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/serf/serf"
	uuid "github.com/satori/go.uuid"
)

// Member is a single member of the cluster.
type Member struct {
	Name   string
	Addr   net.IP
	Tags   map[string]string
	Status MemberStatus
	ID     uuid.UUID

	// bully algorithm fields
	IsLeader bool
}

func (m Member) String() string {
	return fmt.Sprintf("Leader: %v, Name: %s, Addr: %v, Tags: %v, Status: %v, ID: %v", m.IsLeader, m.Name, m.Addr, m.Tags, m.Status, m.ID)
}

// MemberStatus is the state that a member is in.
type MemberStatus int

const (
	StatusNone MemberStatus = iota
	StatusAlive
	StatusLeaving
	StatusLeft
	StatusFailed
)

func (s MemberStatus) String() string {
	switch s {
	case StatusNone:
		return "none"
	case StatusAlive:
		return "alive"
	case StatusLeaving:
		return "leaving"
	case StatusLeft:
		return "left"
	case StatusFailed:
		return "failed"
	default:
		panic(fmt.Sprintf("unknown MemberStatus: %d", s))
	}
}

type memberState struct {
	Member
	WallTime time.Time
	LTime    serf.LamportTime
}

func (ms *memberState) String() string {
	return fmt.Sprintf("%s, Wall: %s, Lamport: %v", ms.Member, ms.WallTime.String(), ms.LTime)
}

// EventType are all the types of events that may occur
type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
	EventMemberUpdate
	EventMemberReap
)

func (t EventType) String() string {
	switch t {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	case EventMemberReap:
		return "member-reap"
	default:
		panic(fmt.Sprintf("unknown event type: %d", t))
	}
}

// MemberEvent is the struct used for member related events.
type MemberEvent struct {
	Type   EventType
	Member Member
}
