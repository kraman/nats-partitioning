package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/kraman/nats-test/lib/discovery"

	"github.com/hashicorp/serf/serf"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

type NatsClusterConfig struct {
	ClusterName     string
	NodeName        string
	NatsConn        *nats.Conn
	Logger          *logrus.Logger
	NodeTTLSec      time.Duration
	ReaperTimeout   time.Duration
	ElectionTimeout time.Duration
}

type NatsCluster struct {
	NatsClusterConfig

	members          map[uuid.UUID]*memberState
	shutdownCh       chan struct{}
	eventCh          chan *discovery.MemberEvent
	lClock           *serf.LamportClock
	bcastChannelName string
	clientID         uuid.UUID
	sync.Mutex

	// election fields
	leaderCh      chan int
	electionTimer *time.Timer
	leaderID      uuid.UUID
}

// Create creates a new Cluster instance, starting all the background tasks
// to maintain membership information.
func Create(config NatsClusterConfig) (discovery.Cluster, error) {
	clientID := uuid.NewV4()

	c := &NatsCluster{
		NatsClusterConfig: config,
		clientID:          clientID,
		members: map[uuid.UUID]*memberState{
			clientID: &memberState{
				Member: discovery.Member{
					Addr:   config.NodeName,
					Tags:   map[string]string{},
					Status: discovery.StatusAlive,
					ID:     clientID,
				},
				WallTime: time.Now(),
			},
		},
		shutdownCh: make(chan struct{}),
		eventCh:    make(chan *discovery.MemberEvent),
		leaderCh:   make(chan int),
		lClock:     &serf.LamportClock{},
	}
	c.bcastChannelName = fmt.Sprintf("_DISCOVERY.%s", c.ClusterName)

	pingCh := make(chan *nats.Msg, 10)
	pingSub, err := c.NatsConn.ChanSubscribe(c.bcastChannelName, pingCh)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to subscript to member broadcast")
	}
	c.lClock.Increment()

	go c.processBroadcast(pingCh, pingSub)

	return c, nil
}

func (c *NatsCluster) Leave() error {
	c.Lock()
	defer c.Unlock()
	if m, ok := c.members[c.clientID]; ok {
		m.Status = discovery.StatusLeaving
	}
	return nil
}

func (c *NatsCluster) Shutdown() error {
	close(c.shutdownCh)
	return nil
}

func (c *NatsCluster) EventChan() <-chan *discovery.MemberEvent {
	return c.eventCh
}

func (c *NatsCluster) Leader() *discovery.Member {
	return &c.members[c.leaderID].Member
}

func (c *NatsCluster) Get(ID uuid.UUID) (*discovery.Member, error) {
	return &c.members[ID].Member, nil
}

func (c *NatsCluster) IsLeader() bool {
	return c.leaderID == c.clientID
}

func (c *NatsCluster) ID() uuid.UUID {
	return c.clientID
}

func (c *NatsCluster) Name() string {
	return c.ClusterName
}
