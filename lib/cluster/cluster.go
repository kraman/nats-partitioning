package cluster

import (
	"fmt"

	uuid "github.com/satori/go.uuid"
)

type Cluster interface {
	// Leave gracefully exits the cluster. It is safe to call this multiple times.
	Leave() error

	// Shutdown forcefully shuts down the member, stopping all network activity and background maintenance associated with the instance.
	//
	// This is not a graceful shutdown, and should be preceded by a call to Leave. Otherwise, other nodes in the cluster will detect this node's exit as a node failure.
	//
	// It is safe to call this method multiple times.
	Shutdown() error

	// EventChan returns a channel that receives all the member events. The events
	// are sent on this channel in proper ordering.
	EventChan() <-chan *MemberEvent

	// Members provides a point-in-time view of cluster members
	Members() []Member

	AliveMembers() []Member

	// Leader returns a point-in-time leader member information
	Leader() Member

	IsLeader() bool

	ID() uuid.UUID

	Name() string
}

type Member struct {
	Addr   interface{}
	Tags   map[string]string
	Status MemberStatus
	ID     uuid.UUID

	// bully algorithm fields
	IsLeader bool
}

func (m Member) String() string {
	return fmt.Sprintf("Leader: %v, Addr: %v, Tags: %v, Status: %v, ID: %v", m.IsLeader, m.Addr, m.Tags, m.Status, m.ID)
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

// EventType are all the types of events that may occur
type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
	EventMemberUpdate
	EventMemberReap
	EventMemberIsLeader
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
	case EventMemberIsLeader:
		return "member-is-leader"
	default:
		panic(fmt.Sprintf("unknown event type: %d", t))
	}
	return ""
}

// MemberEvent is the struct used for member related events.
type MemberEvent struct {
	Type    EventType
	Member  Member
	Members []Member
}
