package nats

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/kraman/nats-test/lib/discovery"
)

func (c *NatsCluster) startElection() {
	c.Logger.Debugf("starting election")
	// stop any previous election timers
	c.stopElection()
	// send election message

	c.electionTimer = time.AfterFunc(c.ElectionTimeout, func() {
		c.leaderCh <- 1
	})
}

func (c *NatsCluster) stopElection() {
	c.Logger.Debug("stopping election on this node")

	if c.electionTimer != nil {
		c.electionTimer.Stop()
		c.electionTimer = nil
	}
}

func (c *NatsCluster) processElectionAnnouncement(m *memberState) {
	c.Lock()
	defer c.Unlock()
	if m.ID == c.clientID || m.Status != discovery.StatusAlive {
		return
	}
	if m.IsLeader {
		c.leaderID = m.ID
		return
	}
	if c.electionTimer != nil {
		// still participating in election
		self := c.members[c.clientID]
		if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == -1 {
			// ID smaller than others. mark as follower
			self.IsLeader = false
			c.stopElection()
		} else {
			// start next cycle of bully election
			c.startElection()
		}
	}
}

func (c *NatsCluster) processElectionMessage(m *memberState) {
	if m.ID == c.clientID {
		return
	}

	self := c.members[c.clientID]
	if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == 1 {
		// ID larger than incoming message. still part of election
		c.sendElectionOK()
		c.startElection()
	}
}

func (c *NatsCluster) processElectionOkMessage(m *memberState) {
	c.Lock()
	defer c.Unlock()
	self := c.members[c.clientID]
	if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == -1 {
		self.IsLeader = false
		c.stopElection()
	}
}

func (c *NatsCluster) sendElectionOK() {
	c.Lock()
	defer c.Unlock()
	memberState := c.members[c.clientID]
	memberState.LTime = c.lClock.Increment()
	memberState.WallTime = time.Now()

	message := &message{
		Type:               okMessage,
		ElectionInProgress: c.electionTimer != nil,
		Member:             memberState,
	}
	b, err := json.Marshal(message)
	if err != nil {
		c.Logger.Errorf("unable to marshal member: %v", err)
		return
	}
	if err := c.NatsConn.Publish(c.bcastChannelName, b); err != nil {
		c.Logger.Errorf("unable to publish to discovery channel: %v", err)
	}
}
