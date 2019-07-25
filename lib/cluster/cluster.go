package cluster

import (
	"fmt"
	"net"
	"time"

	"sync"

	"github.com/hashicorp/serf/serf"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ClusterName     string
	NodeName        string
	NodeIP          net.IP
	NatsConn        *nats.Conn
	Logger          *logrus.Logger
	NodeTTLSec      time.Duration
	ReaperTimeout   time.Duration
	ElectionTimeout time.Duration
}

type Cluster struct {
	Config
	members          map[string]*memberState
	shutdownCh       chan struct{}
	eventCh          chan *MemberEvent
	lClock           *serf.LamportClock
	bcastChannelName string
	sync.Mutex

	// election fields
	electionTimer  *time.Timer
	leaderNodeName string
}

// Create creates a new Cluster instance, starting all the background tasks
// to maintain membership information.
func Create(config Config) (*Cluster, error) {
	cluster := &Cluster{
		Config: config,
		members: map[string]*memberState{
			config.NodeName: &memberState{
				Member: Member{
					Name:   config.NodeName,
					Addr:   config.NodeIP,
					Tags:   map[string]string{},
					Status: StatusAlive,
					ID:     uuid.NewV4(),
				},
				WallTime: time.Now(),
			},
		},
		shutdownCh: make(chan struct{}),
		eventCh:    make(chan *MemberEvent),
		lClock:     &serf.LamportClock{},
	}
	cluster.bcastChannelName = fmt.Sprintf("_DISCOVERY.%s", cluster.ClusterName)

	pingCh := make(chan *nats.Msg, 10)
	pingSub, err := cluster.NatsConn.ChanSubscribe(cluster.bcastChannelName, pingCh)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to subscript to member broadcast")
	}
	cluster.lClock.Increment()

	go cluster.bcastAlive()
	go cluster.processBcast(pingCh, pingSub)

	return cluster, nil
}

// Leave gracefully exits the cluster. It is safe to call this multiple times.
func (c *Cluster) Leave() error {
	c.Lock()
	defer c.Unlock()
	if m, ok := c.members[c.NodeName]; ok {
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
