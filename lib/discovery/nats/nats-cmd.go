package serf

import (
	"os"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"github.com/kraman/nats-test/cmd"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var natsCmd = &cobra.Command{
	Use: "nats",
	RunE: func(cmd *cobra.Command, args []string) error {
		podIP := os.Getenv("POD_IP")
		natsServers, _ := cmd.Flags().GetStringArray("nats-servers")

		logWriter := logrus.StandardLogger().Writer()

		memberlistConfig := memberlist.DefaultLANConfig()
		memberlistConfig.LogOutput = logWriter

		serfEvents := make(chan serf.Event, 16)
		serfConfig := serf.DefaultConfig()
		serfConfig.NodeName = podIP
		serfConfig.EventCh = serfEvents
		serfConfig.MemberlistConfig = memberlistConfig
		serfConfig.LogOutput = logWriter

		serf, err := serf.Create(serfConfig)
		if err != nil {
			defer logWriter.Close()
			return errors.Wrap(err, "unable to create serf cluster")
		}
		
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
	},
}

func init() {
	natsCmd.Flags().StringArray("nats-servers", []string{nats.DefaultURL}, "nats servers")
	cmd.RootCmd.AddCommand(natsCmd)
}
