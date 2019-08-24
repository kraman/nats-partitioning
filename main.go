package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	nc "github.com/kraman/nats-test/lib/cluster/nats"
	"github.com/kraman/nats-test/lib/partitions"

	"github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
)

func main() {
	natsConn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer natsConn.Close()

	ip := os.Getenv("POD_IP")

	logrus.StandardLogger().SetLevel(logrus.DebugLevel)

	cluster, err := nc.Create(nc.NatsClusterConfig{
		ClusterName:     "test",
		NodeName:        ip,
		NatsConn:        natsConn,
		Logger:          logrus.StandardLogger(),
		NodeTTLSec:      1 * time.Second,
		ReaperTimeout:   4 * time.Second,
		ElectionTimeout: 2 * time.Second,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	stanConn, err := stan.Connect(
		"nats",
		strings.ReplaceAll(ip, ".", "-"),
		stan.NatsConn(natsConn),
		stan.SetConnectionLostHandler(
			func(_ stan.Conn, reason error) {
				logrus.Fatalf("Connection lost, reason: %v", reason)
			},
		),
	)
	if err != nil {
		logrus.Fatalf("can't connect to nats-streaming: %v", err)
	}

	manager, err := partitions.NewManager(stanConn, cluster, logrus.StandardLogger())

	t := time.Tick(time.Millisecond * 500)
	i := 0
	for {
		select {
		case <-t:
			manager.Do(3, []byte(fmt.Sprintf("%s.%d", cluster.ID(), i)))
			i++
		}
	}
}
