package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kraman/nats-test/lib/consumergroup"
	"github.com/kraman/nats-test/lib/discovery"
	"github.com/nats-io/stan.go"

	"github.com/LK4D4/trylock"
	"github.com/coreos/etcd/clientv3"
	"github.com/lafikl/consistent"
)

type consumerGroupManager struct {
	consumergroup.Config
	sync.Mutex

	etcdConn             *clientv3.Client
	managerCtx           context.Context
	managerCtxCancelFunc context.CancelFunc

	leaseID            clientv3.LeaseID
	assignedPartitions map[int]string

	partitionLocks map[int]*trylock.Mutex
	subscriptions  map[int]stan.Subscription
}

func NewConsumerGroup(etcdConn *clientv3.Client, config consumergroup.Config) (consumergroup.ConsumerGroup, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	manager := &consumerGroupManager{
		Config:               config,
		etcdConn:             etcdConn,
		managerCtx:           ctx,
		managerCtxCancelFunc: cancelFunc,
		partitionLocks:       map[int]*trylock.Mutex{},
		subscriptions:        map[int]stan.Subscription{},
	}

	grantResp, err := manager.etcdConn.Grant(manager.managerCtx, int64(manager.NodeTTLSec/time.Second))
	if err != nil {
		return nil, err
	}

	manager.leaseID = grantResp.ID
	if _, err := manager.etcdConn.KeepAlive(manager.managerCtx, grantResp.ID); err != nil {
		cancelFunc()
		return nil, err
	}

	go manager.updateOwners()

	return manager, nil
}

func (m *consumerGroupManager) Shutdown() error {
	m.managerCtxCancelFunc()
	return nil
}

func (m *consumerGroupManager) processReleases(msg *clientv3.Event, acquireCh chan int, releaseCh chan int) {
	if msg.Type == clientv3.EventTypeDelete {
		partitionID, err := strconv.Atoi(strings.Split(string(msg.Kv.Key), ".")[2])
		if err != nil {
			m.Logger.Fatalf("unable to parse partition key from event: %v", msg)
		}
		if m.assignedPartitions[partitionID] == m.Cluster.ID().String() && m.partitionLocks[partitionID] == nil {
			m.Logger.Debugf("enqueue %d for acquisition", partitionID)
			acquireCh <- partitionID
		}
	}
}

func (m *consumerGroupManager) processAssigned(assignedInfo []byte, acquireCh chan int, releaseCh chan int) {
	m.assignedPartitions = map[int]string{}
	if err := json.Unmarshal(assignedInfo, &m.assignedPartitions); err != nil {
		m.Logger.Fatalf("unable to unmarshal assigned partititon owners: %v", err)
	}
	// empty out acquire channel
out:
	for {
		select {
		case <-acquireCh:
		default:
			break out
		}
	}

	for partitionKey := range m.partitionLocks {
		if m.assignedPartitions[partitionKey] != m.Cluster.ID().String() {
			m.Logger.Debugf("enqueue %d for release", partitionKey)
			releaseCh <- partitionKey
		}
	}

	for partitionID, owner := range m.assignedPartitions {
		if owner == m.Cluster.ID().String() && m.partitionLocks[partitionID] == nil {
			m.Logger.Debugf("enqueue %d for acquisition", partitionID)
			acquireCh <- partitionID
		}
	}
}

func (m *consumerGroupManager) assignPartitions(msg *discovery.MemberEvent) {
	if !m.Cluster.IsLeader() {
		return
	}

	hashRing := consistent.New()
	for _, member := range msg.Members {
		m.Logger.Debugf("member: %v", member.ID)
		hashRing.Add(member.ID.String())
	}

	assignedOwners := map[int]string{}
	for i := 0; i < m.NumPartitions; i++ {
		node, err := hashRing.Get(strconv.Itoa(i))
		if err != nil {
			m.Logger.Errorf("unable to assign partition %d an owner: %v", i, err)
			return
		}
		assignedOwners[i] = node
	}

	b, err := json.Marshal(assignedOwners)
	if err != nil {
		m.Logger.Fatal(err)
	}

	timeoutCtx, cancelFunc := context.WithTimeout(m.managerCtx, time.Second*5)
	defer cancelFunc()
	getResp, err := m.etcdConn.Get(
		timeoutCtx,
		fmt.Sprintf("partition.%s.assigned", m.Subject),
	)
	if err != nil {
		m.Logger.Fatal(err)
	}

	if getResp.Count != 0 && string(getResp.Kvs[0].Value) == string(b) {
		m.Logger.Debugf("skipping update %s", string(b))
		return
	}
	m.Logger.Debugf("update assignment %s", string(b))

	timeoutCtx, cancelFunc = context.WithTimeout(m.managerCtx, time.Second*5)
	defer cancelFunc()
	_, err = m.etcdConn.Put(
		timeoutCtx,
		fmt.Sprintf("partition.%s.assigned", m.Subject), string(b),
	)
	if err != nil {
		m.Logger.Fatal(err)
	}
}

func (m *consumerGroupManager) updateOwners() {
	doneCh := m.managerCtx.Done()
	memberCh := make(chan *discovery.MemberEvent, 10)
	updatesCh := m.etcdConn.Watch(m.managerCtx, fmt.Sprintf("partition.%s.", m.Subject), clientv3.WithPrefix())
	releaseCh := make(chan int, m.NumPartitions)
	acquireCh := make(chan int, m.NumPartitions)

	m.Cluster.RegisterEventHandler(m.Subject, func(e *discovery.MemberEvent) {
		if m.managerCtx.Err() != nil {
			m.Cluster.UnregisterEventHandler(m.Subject)
			return
		}
		memberCh <- e
	})

	timeoutCtx, cancelFunc := context.WithTimeout(m.managerCtx, time.Second*5)
	defer cancelFunc()
	getResp, err := m.etcdConn.Get(
		timeoutCtx,
		fmt.Sprintf("partition.%s.assigned", m.Subject),
	)
	if err != nil {
		m.Logger.Fatal(err)
	}
	if getResp.Count == 1 {
		m.processAssigned(getResp.Kvs[0].Value, acquireCh, releaseCh)
	}

	for {
		select {
		case partitionID := <-acquireCh:
			m.Logger.Debugf("attempt acquire partititon %d", partitionID)
			timeoutCtx, cancelFunc := context.WithTimeout(m.managerCtx, time.Second*5)
			defer cancelFunc()
			getResp, err := m.etcdConn.Get(
				timeoutCtx,
				fmt.Sprintf("partition.%s.%d", m.Subject, partitionID),
			)
			if err != nil {
				m.Logger.Fatal(err)
			}
			if getResp.Count == 0 {
				m.partitionLocks[partitionID] = &trylock.Mutex{}
				timeoutCtx, cancelFunc := context.WithTimeout(m.managerCtx, time.Second*5)
				defer cancelFunc()

				partName := fmt.Sprintf("partition.%s.%d", m.Subject, partitionID)
				_, err = m.etcdConn.Put(
					timeoutCtx,
					partName, m.Cluster.ID().String(),
					clientv3.WithLease(m.leaseID),
				)
				if err != nil {
					m.Logger.Fatal(err)
				}

				m.partitionLocks[partitionID].Lock()
				sub, err := m.StanConn.QueueSubscribe(
					partName,
					partName,
					m.handleMessage,
					stan.MaxInflight(1),
					stan.SetManualAckMode(),
					stan.DurableName(partName),
				)
				if err != nil {
					m.Logger.Fatal(err)
				}
				m.subscriptions[partitionID] = sub
				m.Logger.Debugf("partition %d acquired", partitionID)
				m.partitionLocks[partitionID].Unlock()
			}
		case partitionID := <-releaseCh:
			m.Logger.Debugf("attempt release partititon %d", partitionID)
			lock, ok := m.partitionLocks[partitionID]
			if ok {
				if lock.TryLock() {
					sub, ok := m.subscriptions[partitionID]
					if ok {
						if err := sub.Close(); err != nil {
							m.Logger.Fatal(err)
						}
					}

					timeoutCtx, cancelFunc := context.WithTimeout(m.managerCtx, time.Second*5)
					defer cancelFunc()
					_, err := m.etcdConn.Delete(
						timeoutCtx,
						fmt.Sprintf("partition.%s.%d", m.Subject, partitionID),
					)
					if err != nil {
						m.Logger.Fatal(err)
					}
					lock.Unlock()
					delete(m.partitionLocks, partitionID)
					m.Logger.Infof("partition %d released", partitionID)
				} else {
					// retry release
					time.AfterFunc(time.Millisecond*500, func() {
						releaseCh <- partitionID
					})
				}
			}
		case etcdMsg := <-updatesCh:
			for _, msg := range etcdMsg.Events {
				if strings.HasSuffix(string(msg.Kv.Key), "assigned") {
					m.processAssigned(msg.Kv.Value, acquireCh, releaseCh)
				} else {
					m.processReleases(msg, acquireCh, releaseCh)
				}
			}
		case msg := <-memberCh:
			m.assignPartitions(msg)
		case <-doneCh:
			return
		}
	}
}

func (m *consumerGroupManager) handleMessage(msg *stan.Msg) {
	m.Logger.Debug("here")
	partitionIDStr := strings.Split(msg.Subject, ".")[2]
	partitionID, _ := strconv.Atoi(partitionIDStr)
	lock := m.partitionLocks[partitionID]
	if !lock.TryLock() {
		return
	}
	defer lock.Unlock()

	m.Handler(partitionIDStr, msg)
	msg.Ack()
}

func (m *consumerGroupManager) Publish(partitionKey string, msg []byte) (err error) {
	hashRing := consistent.New()
	for i := 0; i < m.NumPartitions; i++ {
		hashRing.Add(strconv.Itoa(i))
	}

	partitionIDStr, err := hashRing.Get(partitionKey)
	if err != nil {
		return err
	}

	if err = m.StanConn.Publish(fmt.Sprintf("partition.%s.%s", m.Subject, partitionIDStr), msg); err != nil {
		return err
	}

	return nil
}
