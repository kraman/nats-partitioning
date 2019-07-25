package cluster

import (
	"bytes"
	"encoding/json"
	"time"
)

func (c *Cluster) startElection() {
	c.Logger.Debugf("starting election")
	// stop any previous election timers
	c.stopElection()
	// send election message

	c.electionTimer = time.AfterFunc(c.ElectionTimeout, func() {
		c.Logger.Debug("election self as leader")
		c.stopElection()
		c.Lock()
		defer c.Unlock()
		self := c.members[c.NodeName]
		self.IsLeader = true
		c.leaderNodeName = c.NodeName
		c.postMemberEvent(EventMemberUpdate, c.members[c.NodeName].Member)
	})
}

func (c *Cluster) stopElection() {
	c.Logger.Debug("stopping election on this node")

	if c.electionTimer != nil {
		c.electionTimer.Stop()
		c.electionTimer = nil
	}
}

func (c *Cluster) processElectionAnnouncement(m *memberState) {
	c.Lock()
	defer c.Unlock()
	if m.Name == c.NodeName || m.Status != StatusAlive {
		return
	}
	if m.IsLeader {
		c.leaderNodeName = m.Name
		return
	}
	if c.electionTimer != nil {
		// still participating in election
		self := c.members[c.NodeName]
		if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == -1 {
			self.IsLeader = false
			c.stopElection()
		} else {
			// maybe leader?
			c.startElection()
		}
	}
}

func (c *Cluster) processElectionMessage(m *memberState) {
	if m.Name == c.NodeName {
		return
	}
	self := c.members[c.NodeName]

	if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == 1 {
		// still participating in election, maybe leader?
		c.sendElectionOK()
		c.startElection()
	}
}

func (c *Cluster) processElectionOkMessage(m *memberState) {
	c.Lock()
	defer c.Unlock()
	self := c.members[c.NodeName]
	if bytes.Compare(self.ID.Bytes(), m.ID.Bytes()) == -1 {
		self.IsLeader = false
		c.stopElection()
	}
}

func (c *Cluster) sendElectionOK() {
	c.Lock()
	defer c.Unlock()
	memberState := c.members[c.NodeName]
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
