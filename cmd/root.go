package cmd

import (
	"net"

	"github.com/kraman/nats-test/lib/discovery"
	// "github.com/hashicorp/raft"
	// "github.com/hashicorp/raft-boltdb"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
	"github.com/hashicorp/serf/serf"
)

var RootCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		gossipPort, _ := cmd.Flags().GetInt("gossip-port")
		gossipPeers, _ := cmd.Flags().GetIPSlice("gossip-peers")
		// raftPort, _ := cmd.Flags().GetInt("raft-port")
		// raftDir, _ := cmd.Flags().GetString("raft-dir")
		ip, _ := cmd.Flags().GetIP("ip")

		s, err := discovery.New(ip, ip.String(), gossipPort, gossipPeers)
		if err != nil {
			logrus.Fatal(err)
			return
		}

		for {
			select {
				case ev := <- s.EventChannel():
				switch e:= ev.(type) {
				case serf.MemberEvent:
					logrus.Infof("member event %s. members: %v", e.EventType(), e.Members)
				}
			}
		}

		// cwd, err := os.Getwd()``
		// if err != nil {
		// 	logrus.Fatal(err)
		// }
		// id := fmt.Sprintf("%x", sha512.Sum512([]byte(fmt.Sprintf("%s:%d", ip, raftPort))))
		// if raftDir == "" {
		// 	raftDir = filepath.Join(cwd, id)
		// }
		// if err = os.RemoveAll(raftDir); err != nil {
		// 	logrus.Fatalf("unable to cleanup raft dir: %v", err)
		// }
		// if err = os.MkdirAll(raftDir, 0777); err != nil {
		// 	logrus.Fatalf("unable to create raft dir: %v", err)
		// }
		// raftDBPath := filepath.Join(raftDir, "raft.db")
		// raftDB, err := raftboltdb.NewBoltStore(raftDBPath)
		// if err != nil {
		// 	logrus.Fatalf("unable to open raft DB: %v", err)
		// }

		// snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stdout)
		// if err != nil {
		// 	logrus.Fatalf("unable to open snapshot store", err)
		// }

		// raftAddr := fmt.Sprintf("%s:%d", ip, raftPort)
		// trans, err := raft.NewTCPTransport(raftAddr, nil, 3, 10*time.Second, os.Stdout)
		// if err != nil {
		// 	logrus.Fatalf("unable to create raft transport: %v", err)
		// }

		// c := raft.DefaultConfig()
		// c.LogOutput = os.Stdout
		// c.LocalID = raft.ServerID(id)

		// r, err := raft.NewRaft(c, &fsm{}, raftDB, raftDB, snapshotStore, trans)
		// if err != nil {
		// 	logrus.Fatal(err)
		// }

		// bootstrapConfig := raft.Configuration{
		// 	Servers: []raft.Server{
		// 		{
		// 			Suffrage: raft.Voter,
		// 			ID:       raft.ServerID(id),
		// 			Address:  raft.ServerAddress(raftAddr),
		// 		},
		// 	},
		// }

		// // Add known peers to bootstrap
		// for _, node := range gossipPeers {
		// 	if node.String() == raftAddr {
		// 		continue
		// 	}

		// 	bootstrapConfig.Servers = append(bootstrapConfig.Servers, raft.Server{
		// 		Suffrage: raft.Voter,
		// 		ID:       raft.ServerID(node),
		// 		Address:  raft.ServerAddress(node),
		// 	})
		// }

		// f := r.BootstrapCluster(bootstrapConfig)
		// if err := f.Error(); err != nil {
		// 	logrus.Fatalf("error bootstrapping raft: %s", err)
		// }

		// t := time.Tick(3 * time.Second)

		// for {
		// 	select {
		// 		case <-t:
		// 			future := r.VerifyLeader()

		// 			fmt.Printf("Showing peers known by %s:\n", raftAddr)
		// 			if err = future.Error(); err != nil {
		// 				fmt.Println("Node is a follower")
		// 			} else {
		// 				fmt.Println("Node is leader")
		// 			}

		// 			cfuture := r.GetConfiguration()
		// 			if err = cfuture.Error(); err != nil {
		// 				logrus.Fatalf("error getting config: %s", err)
		// 			}

		// 			configuration := cfuture.Configuration()
		// 			for _, server := range configuration.Servers {
		// 				fmt.Println(server.Address)
		// 			}

		// 		case ev := <-serfEvents:
		// 			leader := r.VerifyLeader()

		// 			if memberEvent, ok := ev.(serf.MemberEvent); ok {
		// 				for _, member := range memberEvent.Members {
		// 					changedPeer := member.Addr.String() + ":" + strconv.Itoa(int(member.Port+1))
		// 					if memberEvent.EventType() == serf.EventMemberJoin {
		// 						if leader.Error() == nil {
		// 							f := r.AddVoter(raft.ServerID(changedPeer), raft.ServerAddress(changedPeer), 0, 0)

		// 							if f.Error() != nil {
		// 								logrus.Fatalf("error adding voter: %s", err)
		// 							}
		// 						}
		// 					} else if memberEvent.EventType() == serf.EventMemberLeave || memberEvent.EventType() == serf.EventMemberFailed || memberEvent.EventType() == serf.EventMemberReap {
		// 						if leader.Error() == nil {
		// 							f := r.RemoveServer(raft.ServerID(changedPeer), 0, 0)
		// 							if f.Error() != nil {
		// 								logrus.Fatalf("error removing server: %s", err)
		// 							}
		// 						}
		// 					}
		// 				}
		// 			}
		// 	}
		// }
	},
}

func init() {
	RootCmd.Flags().IP("ip", net.ParseIP("127.0.0.1"), "IP of this server")
	RootCmd.Flags().IPSlice("gossip-peers", nil, "one or more peer IPs")
	RootCmd.Flags().Int("gossip-port", 1770, "gossip port")
	RootCmd.Flags().Int("raft-port", 1771, "raft port")
	RootCmd.Flags().String("raft-dir", "", "raft data dir")
}

// type fsm struct {
// }

// func (f *fsm) Apply(*raft.Log) interface{} {
// 	return nil
// }

// func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
// 	return nil, nil
// }

// func (f *fsm) Restore(io.ReadCloser) error {
// 	return nil
// }
