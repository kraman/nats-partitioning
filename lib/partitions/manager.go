package partitions

import (
	fmt "fmt"
	"sync"
	"time"

	"github.com/LK4D4/trylock"
	proto "github.com/gogo/protobuf/proto"
	"github.com/kraman/nats-test/lib/cluster"
	"github.com/kraman/nats-test/lib/es"

	stan "github.com/nats-io/stan.go"
	stanpb "github.com/nats-io/stan.go/pb"
	"github.com/sirupsen/logrus"
)

const numPartitions = 10

type Manager interface {
	Do(partID uint32, msg []byte)
}

type manager struct {
	stanConn stan.Conn
	cluster  cluster.Cluster
	logger   *logrus.Logger

	partitions    *sync.Map
	subscriptions map[uint32]stan.Subscription

	workCh chan *stan.Msg
}

func NewManager(stanConn stan.Conn, c cluster.Cluster, logger *logrus.Logger) (Manager, error) {
	m := &manager{
		stanConn:      stanConn,
		cluster:       c,
		partitions:    &sync.Map{},
		subscriptions: map[uint32]stan.Subscription{},
		workCh:        make(chan *stan.Msg, 1),
		logger:        logger,
	}

	go m.processCommands()

	return m, nil
}

func (m *manager) processCommands() {
	ch := make(chan *stan.Msg, 1)
	sub, err := m.stanConn.Subscribe(fmt.Sprintf("_PARTITIONS.%s", m.cluster.Name()), func(m *stan.Msg) {
		ch <- m
	}, stan.MaxInflight(1), stan.StartAt(stanpb.StartPosition_First))
	if err != nil {
		m.logger.Fatal(err)
	}
	defer sub.Close()

	po := &PartitionOwners{
		AssignedOwners: map[uint32]string{},
		ActualOwners:   map[uint32]string{},
	}

	releaseQueue := make(chan uint32, numPartitions)
	for {
		select {
		case evt := <-m.cluster.EventChan():
			m.assignPartitions(evt, po)

		case msg := <-ch:
			// update view with updated events in parition ownership stream
			evt, err := po.Apply(msg)
			if err != nil {
				m.logger.Fatal(err)
			}

			// update or release partitions as needed
			m.reservePartitions(po, evt.Message, releaseQueue)
		case partID := <-releaseQueue:
			// attempt to release partitions
			if ok := m.releasePartitions(po, partID); !ok {
				time.AfterFunc(time.Second, func() {
					releaseQueue <- partID
				})
			}

		case msg := <-m.workCh:
			// do work
			go func() {
				fmt.Printf("ack %v\n", msg)
				msg.Ack()
			}()
		}
	}
}

func (m *manager) assignPartitions(evt *cluster.MemberEvent, po *PartitionOwners) {
	// only leader is allowed to change partition allocation
	if !m.cluster.IsLeader() {
		return
	}

	assignmentUpdated := false
	members := evt.Members
	aliveMembers := []cluster.Member{}
	for _, m := range members {
		if m.Status == cluster.StatusAlive || m.Status == cluster.StatusFailed {
			aliveMembers = append(aliveMembers, m)
		}
	}

	for i := uint32(0); i < numPartitions; i++ {
		// replace with consistent hash later
		assignedMember := members[i%uint32(len(aliveMembers))].ID
		if po.AssignedOwners[i] != assignedMember.String() {
			assignmentUpdated = true
		}
		po.AssignedOwners[i] = assignedMember.String()
	}

	if assignmentUpdated {
		m.logger.Debug("publishing assignment update")
		if err := m.packAndSend(&AssignPartitionsEvent{Owners: po.AssignedOwners}); err != nil {
			m.logger.Errorf("unable to update assignment: %v", err)
			return
		}
	}

	if evt.Type == cluster.EventMemberReap || assignmentUpdated {
		for partID, owner := range po.ActualOwners {
			found := false
			for _, member := range aliveMembers {
				if member.ID.String() == owner {
					found = true
					break
				}
			}
			if !found {
				m.logger.Debugf("releasing part %d for reaped member %s", partID, owner)
				if err := m.packAndSend(&ReleasePartitionEvent{PartitionId: partID, OwnerId: owner}); err != nil {
					m.logger.Fatalf("unable to post release partition %d for reaped member: %v", partID, err)
				}
			}
		}
	}
}

// packAndSend helps encode protobuf and add it to partiton ownership stream
func (m *manager) packAndSend(msg proto.Message) error {
	assignment, err := es.PackEvent(msg)
	if err != nil {
		return err
	}
	return m.stanConn.Publish(fmt.Sprintf("_PARTITIONS.%s", m.cluster.Name()), assignment)
}

func (m *manager) reservePartitions(po *PartitionOwners, evt proto.Message, releaseQueue chan<- uint32) {
	myID := m.cluster.ID().String()
	if msg, ok := evt.(*ReservePartitionEvent); ok {
		if msg.OwnerId == myID {
			partID := msg.PartitionId
			mutex := &trylock.Mutex{}
			m.partitions.Store(partID, mutex)
			mutex.Lock()
			defer mutex.Unlock()

			sub, err := m.stanConn.QueueSubscribe(
				fmt.Sprintf("_PARTITIONS.%s.%d", m.cluster.Name(), partID),
				fmt.Sprintf("%s.%d", m.cluster.Name(), partID),
				func(msg *stan.Msg) {
					mutex, ok := m.partitions.Load(partID)
					if ok {
						mutex := mutex.(*trylock.Mutex)
						mutex.Lock()
						defer mutex.Unlock()
						m.workCh <- msg
					}
				},
				stan.SetManualAckMode(),
				stan.MaxInflight(1),
				stan.StartWithLastReceived(),
				stan.AckWait(time.Second*2),
			)
			if err != nil {
				m.logger.Fatalf("unable to subscribe to partition %d: %v", msg.PartitionId, err)
			}
			m.subscriptions[msg.PartitionId] = sub
			return
		}
	}

	if evt, ok := evt.(*ReleasePartitionEvent); ok {
		partID := evt.PartitionId
		if po.AssignedOwners[partID] == myID {
			m.logger.Infof("reserve partition %d", partID)
			if err := m.packAndSend(&ReservePartitionEvent{PartitionId: partID, OwnerId: myID}); err != nil {
				m.logger.Fatalf("unable to reserve partition %d: %v", partID, err)
				return
			}
		}
	}

	if _, ok := evt.(*AssignPartitionsEvent); ok {
		for partID, owner := range po.AssignedOwners {
			if owner == myID {
				if po.ActualOwners[partID] == myID {
					continue
				}

				if owner, ok := po.ActualOwners[partID]; ok && owner != myID {
					m.logger.Infof("wait for other node to release partition %d", partID)
					continue
				}
				m.logger.Infof("reserve partition %d", partID)
				if err := m.packAndSend(&ReservePartitionEvent{PartitionId: partID, OwnerId: myID}); err != nil {
					m.logger.Errorf("unable to reserve partition %d: %v", partID, err)
					return
				}
			} else {
				if po.ActualOwners[partID] == myID {
					m.logger.Printf("enqueue %d for release (%s, %s)", partID, owner, myID)
					releaseQueue <- partID
				}
			}
		}
	}
}

func (m *manager) releasePartitions(po *PartitionOwners, partID uint32) bool {
	myID := m.cluster.ID().String()
	if po.AssignedOwners[partID] != myID && po.ActualOwners[partID] == myID {
		if lock, ok := m.partitions.Load(partID); ok {
			lock := lock.(*trylock.Mutex)
			if !lock.TryLock() {
				m.logger.Debugf("partition locked. waiting before releasing")
				return false
			}
			defer lock.Unlock()
			if err := m.subscriptions[partID].Close(); err != nil {
				m.logger.Errorf("error while closing connection for partition %d: %v", partID, err)
			}
			m.partitions.Delete(partID)
			delete(m.subscriptions, partID)
		}

		m.logger.Debugf("releasing partition %d", partID)
		if err := m.packAndSend(&ReleasePartitionEvent{PartitionId: partID, OwnerId: myID}); err != nil {
			m.logger.Fatalf("unable to send release message: %v", err)
			return false
		}
	}
	return true
}

func (m *manager) Do(partID uint32, msg []byte) {
	if err := m.stanConn.Publish(fmt.Sprintf("_PARTITIONS.%s.%d", m.cluster.Name(), partID), msg); err != nil {
		m.logger.Println(err)
	}
}
