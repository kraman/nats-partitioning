package main

import (
	"net"
	"os"
	"time"

	"github.com/kraman/nats-test/lib/cluster"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer nc.Close()
	ip := os.Getenv("POD_IP")

	_, err = cluster.Create(cluster.Config{
		ClusterName:     "test",
		NodeName:        ip,
		NodeIP:          net.ParseIP(ip),
		NatsConn:        nc,
		Logger:          logrus.StandardLogger(),
		NodeTTLSec:      2 * time.Second,
		ReaperTimeout:   10 * time.Second,
		ElectionTimeout: 10 * time.Second,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	select {}
}
