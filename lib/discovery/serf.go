package discovery

import (
	"net"
	"io"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"github.com/sirupsen/logrus"
	"github.com/pkg/errors"
)

type SerfCluster struct {
	*serf.Serf
	logWriter io.WriteCloser
	serfEvents chan serf.Event
}

func (c *SerfCluster) EventChannel() (<-chan serf.Event) {
	return c.serfEvents
}

func (c *SerfCluster) Shutdown() error {
	if err := c.logWriter.Close(); err != nil {
		logrus.Errorf("unable to close log writer: %v", err)
	}
	return c.Serf.Shutdown()
}

func New(ip net.IP, nodeName string, gossipPort int, gossipPeers []net.IP) (*SerfCluster, error) {
	// raftPort, _ := cmd.Flags().GetInt("raft-port")
	// raftDir, _ := cmd.Flags().GetString("raft-dir")

	
	logWriter := logrus.StandardLogger().Writer()

	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.BindAddr = ip.String()
	memberlistConfig.BindPort = gossipPort
	memberlistConfig.LogOutput = logWriter

	serfEvents := make(chan serf.Event, 16)
	serfConfig := serf.DefaultConfig()
	serfConfig.NodeName = nodeName
	serfConfig.EventCh = serfEvents
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = logWriter


	serf, err := serf.Create(serfConfig)
	if err != nil {
		defer logWriter.Close()
		return nil, errors.Wrap(err, "unable to create serf cluster")
	}
	s := &SerfCluster{logWriter: logWriter, serfEvents: serfEvents, Serf: serf}

	// Join an existing cluster by specifying at least one known member.
	if len(gossipPeers) > 0 {
		p := make([]string, len(gossipPeers))
		for i := range gossipPeers {
			p[i] = gossipPeers[i].String()
		}
		numPeers, err := s.Serf.Join(p, false)
		if err != nil {
			defer s.Shutdown()
			return nil, errors.Wrap(err, "unable to join peers")
		}
		logrus.Infof("joined %d peers", numPeers)
	}
	return s, nil
}