package cluster

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	nats "github.com/nats-io/nats.go"
)

func (c *Cluster) postMemberEvent(t EventType, m Member) {
	c.Logger.Infof("post member event %v %v", t, m)
	select {
	case c.eventCh <- &MemberEvent{Type: EventMemberLeave, Member: m}:
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

func (c *Cluster) bcastAlive() {
	c.postMemberEvent(EventMemberJoin, c.members[c.NodeName].Member)

	t := time.Tick(c.NodeTTLSec / 2)
	for {
		select {
		case <-t:
			c.Lock()
			memberState := c.members[c.NodeName]
			memberState.LTime = c.lClock.Increment()
			memberState.WallTime = time.Now()

			message := &message{
				Type:               keepAliveMessage,
				ElectionInProgress: c.electionTimer != nil,
				Member:             memberState,
			}
			b, err := json.Marshal(message)
			c.Unlock()
			if err != nil {
				c.Logger.Errorf("unable to marshal member: %v", err)
				continue
			}
			if err := c.NatsConn.Publish(c.bcastChannelName, b); err != nil {
				c.Logger.Errorf("unable to publish to discovery channel: %v", err)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Cluster) processBcast(pingCh chan *nats.Msg, pingSub *nats.Subscription) {
	defer pingSub.Unsubscribe()
	t := time.Tick(c.NodeTTLSec)
	gossipSettleTimer := time.NewTimer(c.NodeTTLSec)
	for {
		select {
		case <-gossipSettleTimer.C:
			for _, m := range c.members {
				if m.Status == StatusAlive && m.IsLeader {
					c.leaderNodeName = m.Name
				}
			}
			if c.leaderNodeName == "" {
				c.startElection()
			}
		case <-t:
			cmp := time.Now().Add(-c.NodeTTLSec)
			c.Lock()
			for _, m := range c.members {
				if m.WallTime.Before(cmp) {
					if m.IsLeader {
						c.members[m.Name].IsLeader = false
						c.leaderNodeName = ""
						c.startElection()
					}
					switch m.Status {
					case StatusLeaving:
						m.Status = StatusLeft
						c.postMemberEvent(EventMemberLeave, m.Member)
					case StatusLeft:
						fallthrough
					case StatusFailed:
						if m.WallTime.Before(time.Now().Add(-time.Second * 30)) {
							c.postMemberEvent(EventMemberReap, m.Member)
							delete(c.members, m.Name)
						}
					default:
						m.Status = StatusFailed
						c.postMemberEvent(EventMemberFailed, m.Member)
					}
				}
			}
			c.Unlock()
		case pkt := <-pingCh:
			message := &message{}

			err := json.Unmarshal(pkt.Data, message)
			if err != nil {
				c.Logger.Errorf("unable to unmarshal member: %v", err)
				continue
			}
			c.Logger.Debugf("revieve msg %v", message)

			switch message.Type {
			case keepAliveMessage:
				c.Lock()
				if message.Member.Name != c.NodeName {
					c.lClock.Witness(message.Member.LTime)
				}

				if cm, ok := c.members[message.Member.Name]; !ok {
					message.Member.WallTime = time.Now()
					c.members[message.Member.Name] = message.Member
					c.postMemberEvent(EventMemberJoin, message.Member.Member)
				} else {
					cm.WallTime = time.Now()
					if !reflect.DeepEqual(cm.Tags, message.Member.Tags) || cm.Status != message.Member.Status || cm.IsLeader != message.Member.IsLeader {
						cm.Tags = message.Member.Tags
						cm.Status = message.Member.Status
						cm.IsLeader = message.Member.IsLeader
						c.postMemberEvent(EventMemberUpdate, cm.Member)
					}
				}
				c.Unlock()

				if message.Member.Name != c.NodeName && message.ElectionInProgress {
					c.processElectionAnnouncement(message.Member)
				}
			case electionMessage:
				c.processElectionMessage(message.Member)
			case okMessage:
				c.processElectionOkMessage(message.Member)
			}
		case <-c.shutdownCh:
			return
		}
	}
}
