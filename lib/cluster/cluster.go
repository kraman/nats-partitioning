package cluster

import (
	"fmt"
	"net"
	"reflect"
	"time"

	"encoding/json"

	"sync"

	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Member is a single member of the cluster.
type Member struct {
	Name   string
	Addr   net.IP
	Tags   map[string]string
	Status MemberStatus
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

type memberState struct {
	Member
	wallTime time.Time
}

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

type Cluster struct {
	clusterName string
	nodeName    string
	nc          *nats.Conn
	logger      *logrus.Logger
	members     map[string]*memberState
	shutdownCh  chan struct{}
	eventCh     chan *MemberEvent
	sync.Mutex
}

// Create creates a new Cluster instance, starting all the background tasks
// to maintain membership information.
func Create(nc *nats.Conn, clusterName, nodeName, nodeIP string) (*Cluster, error) {
	cluster := &Cluster{
		clusterName: clusterName,
		nodeName:    nodeName,
		nc:          nc,
		logger:      logrus.StandardLogger(),
		members: map[string]*memberState{
			nodeName: &memberState{
				Member: Member{
					Name:   nodeName,
					Addr:   net.ParseIP(nodeIP),
					Tags:   map[string]string{},
					Status: StatusAlive,
				},
				wallTime: time.Now(),
			},
		},
		shutdownCh: make(chan struct{}),
		eventCh:    make(chan *MemberEvent),
	}

	bcastChannelName := fmt.Sprintf("_DISCOVERY.%s", cluster.clusterName)
	pingCh := make(chan *nats.Msg, 10)
	pingSub, err := nc.ChanSubscribe(bcastChannelName, pingCh)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to subscript to member broadcast")
	}

	go cluster.bcastAlive(bcastChannelName)
	go cluster.processBcast(bcastChannelName, pingCh, pingSub)

	return cluster, nil
}

func (c *Cluster) bcastAlive(bcastChannelName string) {
	t := time.Tick(time.Second)
	for {
		select {
		case <-t:
			c.Lock()
			b, err := json.Marshal(c.members[c.nodeName])
			c.Unlock()
			if err != nil {
				c.logger.Errorf("unable to marshal member: %v", err)
				continue
			}
			if err := c.nc.Publish(bcastChannelName, b); err != nil {
				c.logger.Errorf("unable to publish to discovery channel: %v", err)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Cluster) processBcast(bcastChannelName string, pingCh chan *nats.Msg, pingSub *nats.Subscription) {
	defer pingSub.Unsubscribe()
	t := time.Tick(time.Second * 2)
	for {
		select {
		case <-t:
			cmp := time.Now().Add(-time.Second * 2)
			c.Lock()
			for _, m := range c.members {
				if m.wallTime.Before(cmp) {
					switch m.Status {
					case StatusLeaving:
						m.Status = StatusLeft
						select {
						case c.eventCh <- &MemberEvent{Type: EventMemberLeave, Member: m.Member}:
						default:
						}
					case StatusLeft:
						fallthrough
					case StatusFailed:
						if m.wallTime.Before(time.Now().Add(-time.Second * 30)) {
							select {
							case c.eventCh <- &MemberEvent{Type: EventMemberReap, Member: m.Member}:
							default:
							}
							delete(c.members, m.Name)
						}
					default:
						m.Status = StatusFailed
						select {
						case c.eventCh <- &MemberEvent{Type: EventMemberFailed, Member: m.Member}:
						default:
						}
					}
				}
			}
			c.Unlock()
		case msg := <-pingCh:
			m := &Member{}
			err := json.Unmarshal(msg.Data, m)
			if err != nil {
				c.logger.Errorf("unable to unmarshal member: %v", err)
				continue
			}
			c.Lock()
			if cm, ok := c.members[m.Name]; !ok {
				select {
				case c.eventCh <- &MemberEvent{Type: EventMemberJoin, Member: *m}:
				default:
				}
			} else {
				if !reflect.DeepEqual(cm.Tags, m.Tags) || cm.Status != m.Status {
					select {
					case c.eventCh <- &MemberEvent{Type: EventMemberUpdate, Member: *m}:
					default:
					}
				}
			}
			c.members[m.Name] = &memberState{
				Member:   *m,
				wallTime: time.Now(),
			}
			c.Unlock()
		case <-c.shutdownCh:
			return
		}
	}
}

// Leave gracefully exits the cluster. It is safe to call this multiple times.
func (c *Cluster) Leave() error {
	c.Lock()
	defer c.Unlock()
	if m, ok := c.members[c.nodeName]; ok {
		m.Status = StatusLeaving
	}
	return nil
}

// Shutdown forcefully shuts down the member, stopping all network activity and background maintenance associated with the instance.
//
// This is not a graceful shutdown, and should be preceded by a call to Leave. Otherwise, other nodes in the cluster will detect this node's exit as a node failure.
//
// It is safe to call this method multiple times.
func (c *Cluster) Shutdown() error {
	close(c.shutdownCh)
	return nil
}

func (c *Cluster) EventChan() <-chan *MemberEvent {
	return c.eventCh
}

func (c *Cluster) Members() []Member {
	m := []Member{}
	for _, ms := range c.members {
		if ms.Status != StatusLeft {
			m = append(m, ms.Member)
		}
	}
	return m
}
