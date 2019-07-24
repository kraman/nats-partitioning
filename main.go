package main

import (
	"fmt"
	"os"

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
	cluster, err := cluster.Create(nc, "test", ip, ip)
	if err != nil {
		logrus.Fatal(err)
	}

	for {
		e := <-cluster.EventChan()
		fmt.Printf("%v\n", e)
	}
}
