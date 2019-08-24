package nats

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/kraman/nats-test/lib/cluster"

	nats "github.com/nats-io/nats.go"
	uuid "github.com/satori/go.uuid"
)

func (c *NatsCluster) postMemberEvent(t cluster.EventType, m cluster.Member) {
	c.Logger.Debugf("post member event %v %v", t, m)
	select {
	case c.eventCh <- &cluster.MemberEvent{Type: t, Member: m, Members: c.Members()}:
	default:
	}
}

type messageType int

const (
	keepAliveMessage messageType = iota
	okMessage
	electionMessage
)

func (t messageType) String() string {
	switch t {
	case keepAliveMessage:
		return "keep-alive"
	case okMessage:
		return "election-ok"
	case electionMessage:
		return "election"
	}
	panic("unknown message type")
}

type message struct {
	Type               messageType
	ElectionInProgress bool
	Member             *memberState
}

func (m message) String() string {
	return fmt.Sprintf("Type: %s, In Election: %v, Member: { %s }", m.Type, m.ElectionInProgress, m.Member)
}

func (c *NatsCluster) processBroadcast(msgCh chan *nats.Msg, pingSub *nats.Subscription) {
	defer pingSub.Unsubscribe()
	t := time.Tick(c.NodeTTLSec)
	gossipSettleTimer := time.NewTimer(c.NodeTTLSec)
	bcastTick := time.Tick(c.NodeTTLSec / 2)
	c.postMemberEvent(cluster.EventMemberJoin, c.members[c.clientID].Member)

	for {
		select {
		case <-bcastTick:
			// boaadcast keepalive message
			memberState := c.members[c.clientID]
			memberState.LTime = c.lClock.Increment()
			memberState.WallTime = time.Now()

			message := &message{
				Type:               keepAliveMessage,
				ElectionInProgress: false,
				Member:             memberState,
			}
			b, err := json.Marshal(message)
			if err != nil {
				c.Logger.Errorf("unable to marshal member: %v", err)
				continue
			}
			if err := c.NatsConn.Publish(c.bcastChannelName, b); err != nil {
				c.Logger.Errorf("unable to publish to discovery channel: %v", err)
			}
		case <-gossipSettleTimer.C:
			// trigger election and update election leader result
			for _, m := range c.members {
				if m.Status == cluster.StatusAlive && m.IsLeader {
					c.leaderID = m.ID
				}
			}
			if c.leaderID == uuid.Nil {
				c.startElection()
			}
		case <-t:
			// update member liveness status based on keepalive pings
			cmp := time.Now().Add(-c.NodeTTLSec)
			for _, m := range c.members {
				if m.WallTime.Before(cmp) {
					if m.IsLeader {
						c.members[m.ID].IsLeader = false
						c.leaderID = uuid.Nil
						c.startElection()
					}
					switch m.Status {
					case cluster.StatusLeaving:
						m.Status = cluster.StatusLeft
						c.postMemberEvent(cluster.EventMemberLeave, m.Member)
					case cluster.StatusLeft:
						fallthrough
					case cluster.StatusFailed:
						if m.WallTime.Before(time.Now().Add(-c.ReaperTimeout)) {
							c.postMemberEvent(cluster.EventMemberReap, m.Member)
							delete(c.members, m.ID)
						}
					default:
						m.Status = cluster.StatusFailed
						c.postMemberEvent(cluster.EventMemberFailed, m.Member)
					}
				}
			}
		case pkt := <-msgCh:
			// process events from nats
			message := &message{}

			err := json.Unmarshal(pkt.Data, message)
			if err != nil {
				c.Logger.Errorf("unable to unmarshal member: %v", err)
				continue
			}
			c.Logger.Tracef("revieve msg %v", message)

			switch message.Type {
			case keepAliveMessage:
				if message.Member.ID != c.clientID {
					c.lClock.Witness(message.Member.LTime)
				}

				if cm, ok := c.members[message.Member.ID]; !ok {
					message.Member.WallTime = time.Now()
					c.members[message.Member.ID] = message.Member
					c.postMemberEvent(cluster.EventMemberJoin, message.Member.Member)
				} else {
					cm.WallTime = time.Now()
					if !reflect.DeepEqual(cm.Tags, message.Member.Tags) || cm.Status != message.Member.Status || cm.IsLeader != message.Member.IsLeader {
						cm.Tags = message.Member.Tags
						cm.Status = message.Member.Status
						cm.IsLeader = message.Member.IsLeader
						c.postMemberEvent(cluster.EventMemberUpdate, cm.Member)
					}
				}

				if message.Member.ID != c.clientID && message.ElectionInProgress {
					c.processElectionAnnouncement(message.Member)
				}
			case electionMessage:
				c.processElectionMessage(message.Member)
			case okMessage:
				c.processElectionOkMessage(message.Member)
			}
		case <-c.leaderCh:
			// set self as leader (end of bully algorithm)
			c.Logger.Debug("election self as leader")
			c.stopElection()
			c.Lock()
			defer c.Unlock()
			self := c.members[c.clientID]
			self.IsLeader = true
			c.leaderID = c.clientID
			c.postMemberEvent(cluster.EventMemberIsLeader, c.members[c.clientID].Member)
		case <-c.shutdownCh:
			// exit loop
			return
		}
	}
}
