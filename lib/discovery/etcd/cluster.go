package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kraman/nats-test/lib/discovery"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

type EtcdClusterConfig struct {
	ClusterName string
	NodeName    string
	EtcdConn    *clientv3.Client
	Logger      *logrus.Logger
	NodeTTLSec  time.Duration
}

type EtcdCluster struct {
	EtcdClusterConfig
	sync.Mutex

	eventCh              chan *discovery.MemberEvent
	eventHandlers        *sync.Map
	clusterCtx           context.Context
	clusterCtxCancelFunc context.CancelFunc
	self                 *discovery.Member
	leaderElection       *concurrency.Election

	members map[string]*discovery.Member
}

func Create(config EtcdClusterConfig) (discovery.Cluster, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	c := &EtcdCluster{
		EtcdClusterConfig:    config,
		eventCh:              make(chan *discovery.MemberEvent),
		eventHandlers:        &sync.Map{},
		clusterCtx:           ctx,
		clusterCtxCancelFunc: cancelFunc,
		self: &discovery.Member{
			ID:       uuid.NewV4(),
			Addr:     config.NodeName,
			Tags:     map[string]string{},
			Status:   discovery.StatusAlive,
			IsLeader: false,
		},
		members: map[string]*discovery.Member{},
	}

	selfJSON, err := json.Marshal(c.self)
	if err != nil {
		return nil, err
	}

	grantResp, err := c.EtcdConn.Grant(c.clusterCtx, int64(c.NodeTTLSec/time.Second))
	if err != nil {
		return nil, err
	}

	if _, err := c.EtcdConn.KeepAlive(c.clusterCtx, grantResp.ID); err != nil {
		cancelFunc()
		return nil, err
	}

	session, err := concurrency.NewSession(c.EtcdConn, concurrency.WithLease(grantResp.ID))
	if err != nil {
		cancelFunc()
		return nil, err
	}

	c.leaderElection = concurrency.NewElection(session, fmt.Sprintf("cluster.%s-leader", c.ClusterName))
	updatesCh := c.EtcdConn.Watch(clientv3.WithRequireLeader(c.clusterCtx), fmt.Sprintf("cluster.%s.", c.ClusterName), clientv3.WithPrefix())
	electionCh := c.leaderElection.Observe(c.clusterCtx)

	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	_, err = c.EtcdConn.Put(
		timeoutCtx,
		fmt.Sprintf("cluster.%s.%s", c.ClusterName, c.self.ID.String()),
		string(selfJSON), clientv3.WithLease(grantResp.ID))
	if err != nil {
		logrus.Fatal(err)
	}

	go c.watchMembers(updatesCh, electionCh)
	time.Sleep(time.Second)

	go func() {
		if err := c.leaderElection.Campaign(c.clusterCtx, c.self.ID.String()); err != nil {
			logrus.Fatal(err)
		}
	}()

	go func() {
		for {
			select {
			case e := <-c.eventCh:
				c.eventHandlers.Range(func(_ interface{}, h interface{}) bool {
					h.(discovery.EventHandler)(e)
					return true
				})
			case <-c.clusterCtx.Done():
				return
			}
		}
	}()

	return c, nil
}

func (c *EtcdCluster) watchMembers(updatesCh clientv3.WatchChan, electionCh <-chan clientv3.GetResponse) {
	c.updateView()
	for {
		select {
		case watchEvent := <-updatesCh:
			for _, ev := range watchEvent.Events {
				c.Lock()
				if ev.Type == clientv3.EventTypePut {
					member := &discovery.Member{}
					if err := json.Unmarshal(ev.Kv.Value, member); err != nil {
						logrus.Fatal(err)
					}
					c.members[string(ev.Kv.Key)] = member

					members := []discovery.Member{}
					for k := range c.members {
						members = append(members, *c.members[k])
					}

					if ev.IsCreate() {
						logrus.Infof("member-join %v", member)
						c.eventCh <- &discovery.MemberEvent{
							Type:    discovery.EventMemberJoin,
							Member:  *member,
							Members: members,
						}
					} else {
						logrus.Infof("member-update %v", member)
						c.eventCh <- &discovery.MemberEvent{
							Type:    discovery.EventMemberUpdate,
							Member:  *member,
							Members: members,
						}
					}
				} else if ev.Type == clientv3.EventTypeDelete {
					deletedMember := c.members[string(ev.Kv.Key)]
					deletedMember.Status = discovery.StatusLeft
					logrus.Infof("member-reap %v", deletedMember)
					delete(c.members, string(ev.Kv.Key))

					members := []discovery.Member{}
					for k := range c.members {
						members = append(members, *c.members[k])
					}
					c.eventCh <- &discovery.MemberEvent{
						Type:    discovery.EventMemberReap,
						Member:  *deletedMember,
						Members: members,
					}
				}
				c.Unlock()
			}
		case <-electionCh:
			c.updateView()
		}
	}
}

func (c *EtcdCluster) Leave() error {
	c.clusterCtxCancelFunc()
	return nil
}

func (c *EtcdCluster) Shutdown() error {
	return c.Leave()
}

func (c *EtcdCluster) updateView() error {
	c.Lock()
	defer c.Unlock()

	timeoutCtx, cancelFunc := context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()
	resp, err := c.EtcdConn.KV.Get(timeoutCtx, fmt.Sprintf("cluster.%s.", c.ClusterName), clientv3.WithPrefix())
	if err != nil {
		return err
	}

	timeoutCtx, cancelFunc = context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()
	leaderResp, err := c.leaderElection.Leader(timeoutCtx)
	if err != nil {
		return err
	}

	members := map[string]*discovery.Member{}
	for _, v := range resp.Kvs {
		member := &discovery.Member{}
		if err := json.Unmarshal(v.Value, member); err != nil {
			return err
		}
		if string(leaderResp.Kvs[0].Value) == member.ID.String() {
			member.IsLeader = true
		}
		members[string(v.Key)] = member
	}

	c.members = members
	return nil
}

func (c *EtcdCluster) Leader() *discovery.Member {
	c.Lock()
	defer c.Unlock()
	for _, m := range c.members {
		if m.IsLeader {
			return m
		}
	}
	return nil
}

// Get returns point-in-time member information
func (c *EtcdCluster) Members() ([]discovery.Member, error) {
	timeoutCtx, cancelFunc := context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()

	memberResp, err := c.EtcdConn.Get(timeoutCtx, fmt.Sprintf("cluster.%s.", c.ClusterName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	members := make([]discovery.Member, memberResp.Count)
	for i, kv := range memberResp.Kvs {
		member := discovery.Member{}
		if err := json.Unmarshal(kv.Value, &member); err != nil {
			return nil, err
		}
		members[i] = member
	}

	return members, nil
}

func (c *EtcdCluster) IsLeader() bool {
	timeoutCtx, cancelFunc := context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()
	leaderResp, err := c.leaderElection.Leader(timeoutCtx)
	if err != nil {
		return false
	}

	if string(leaderResp.Kvs[0].Value) == c.self.ID.String() {
		return true
	}
	return false
}

func (c *EtcdCluster) ID() uuid.UUID {
	return c.self.ID
}

func (c *EtcdCluster) Name() string {
	return c.ClusterName
}

func (c *EtcdCluster) RegisterEventHandler(name string, h discovery.EventHandler) {
	c.eventHandlers.Store(name, h)
	if members, err := c.Members(); err == nil && len(members) > 0 {
		h(&discovery.MemberEvent{Type: discovery.EventMemberUpdate, Members: members})
	}
}

func (c *EtcdCluster) UnregisterEventHandler(name string) {
	c.eventHandlers.Delete(name)
}
