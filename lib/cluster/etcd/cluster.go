package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kraman/nats-test/lib/cluster"

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

	eventCh              chan *cluster.MemberEvent
	clusterCtx           context.Context
	clusterCtxCancelFunc context.CancelFunc
	self                 *cluster.Member
	leaderElection       *concurrency.Election

	members map[string]*cluster.Member
}

func Create(config EtcdClusterConfig) (cluster.Cluster, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	c := &EtcdCluster{
		EtcdClusterConfig:    config,
		eventCh:              make(chan *cluster.MemberEvent),
		clusterCtx:           ctx,
		clusterCtxCancelFunc: cancelFunc,
		self: &cluster.Member{
			ID:       uuid.NewV4(),
			Addr:     config.NodeName,
			Tags:     map[string]string{},
			Status:   cluster.StatusAlive,
			IsLeader: false,
		},
		members: map[string]*cluster.Member{},
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

	c.leaderElection = concurrency.NewElection(session, fmt.Sprintf("cluster/%s-leader", c.ClusterName))
	updatesCh := c.EtcdConn.Watch(clientv3.WithRequireLeader(c.clusterCtx), fmt.Sprintf("cluster/%s/", c.ClusterName), clientv3.WithPrefix())
	electionCh := c.leaderElection.Observe(c.clusterCtx)

	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	_, err = c.EtcdConn.Put(
		timeoutCtx,
		fmt.Sprintf("cluster/%s/%s", c.ClusterName, c.self.ID.String()),
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
					member := &cluster.Member{}
					if err := json.Unmarshal(ev.Kv.Value, member); err != nil {
						logrus.Fatal(err)
					}
					c.members[string(ev.Kv.Key)] = member

					members := []cluster.Member{}
					for k := range c.members {
						members = append(members, *c.members[k])
					}

					if ev.IsCreate() {
						logrus.Infof("member-join %v", member)
						c.eventCh <- &cluster.MemberEvent{
							Type:    cluster.EventMemberJoin,
							Member:  *member,
							Members: members,
						}
					} else {
						logrus.Infof("member-update %v", member)
						c.eventCh <- &cluster.MemberEvent{
							Type:    cluster.EventMemberUpdate,
							Member:  *member,
							Members: members,
						}
					}
				} else if ev.Type == clientv3.EventTypeDelete {
					deletedMember := c.members[string(ev.Kv.Key)]
					deletedMember.Status = cluster.StatusLeft
					logrus.Infof("member-reap %v", deletedMember)
					delete(c.members, string(ev.Kv.Key))

					members := []cluster.Member{}
					for k := range c.members {
						members = append(members, *c.members[k])
					}
					c.eventCh <- &cluster.MemberEvent{
						Type:    cluster.EventMemberReap,
						Member:  *deletedMember,
						Members: members,
					}
				}
				c.Unlock()
			}
		case leaderUpdate := <-electionCh:
			c.Lock()
			leaderID := leaderUpdate.Kvs[0].Value
			var leader *cluster.Member
			members := []cluster.Member{}
			for k := range c.members {
				c.members[k].IsLeader = c.members[k].ID.String() == string(leaderID)
				if c.members[k].IsLeader {
					leader = c.members[k]
				}
				members = append(members, *c.members[k])
			}
			logrus.Infof("leader-elected %v", leader)
			c.eventCh <- &cluster.MemberEvent{
				Type:    cluster.EventMemberIsLeader,
				Member:  *leader,
				Members: members,
			}
			c.Unlock()
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

func (c *EtcdCluster) EventChan() <-chan *cluster.MemberEvent {
	return c.eventCh
}

func (c *EtcdCluster) updateView() error {
	c.Lock()
	defer c.Unlock()

	timeoutCtx, cancelFunc := context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()
	resp, err := c.EtcdConn.KV.Get(timeoutCtx, fmt.Sprintf("cluster/%s/", c.ClusterName), clientv3.WithPrefix())
	if err != nil {
		return err
	}

	timeoutCtx, cancelFunc = context.WithTimeout(c.clusterCtx, time.Second*5)
	defer cancelFunc()
	leaderResp, err := c.leaderElection.Leader(timeoutCtx)
	if err != nil {
		return err
	}

	members := map[string]*cluster.Member{}
	for _, v := range resp.Kvs {
		member := &cluster.Member{}
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

func (c *EtcdCluster) Leader() *cluster.Member {
	c.Lock()
	defer c.Unlock()
	for _, m := range c.members {
		if m.IsLeader {
			return m
		}
	}
	return nil
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
