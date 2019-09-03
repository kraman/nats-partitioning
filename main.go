package main

import (
	"os"
	"time"

	"github.com/kraman/nats-test/lib/cluster"
	ec "github.com/kraman/nats-test/lib/cluster/etcd"
	nc "github.com/kraman/nats-test/lib/cluster/nats"
	etcdm "github.com/kraman/nats-test/lib/manager/etcd"

	"github.com/coreos/etcd/clientv3"
	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"

	_ "github.com/jackc/pgx"
	_ "github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func main() {
	ip := os.Getenv("POD_IP")

	logrus.StandardLogger().SetLevel(logrus.InfoLevel)
	var c cluster.Cluster
	var etcdClient *clientv3.Client
	var err error

	if false {
		natsConn, err := nats.Connect(nats.DefaultURL)
		if err != nil {
			logrus.Fatal(err)
		}
		defer natsConn.Close()
		c, err = nc.Create(nc.NatsClusterConfig{
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
	} else {
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{"http://127.0.0.1:2379"},
			DialTimeout: 2 * time.Second,
		})
		if err != nil {
			logrus.Fatal(err)
		}
		c, err = ec.Create(ec.EtcdClusterConfig{
			ClusterName: "test",
			NodeName:    ip,
			EtcdConn:    etcdClient,
			Logger:      logrus.StandardLogger(),
			NodeTTLSec:  1 * time.Second,
		})
		if err != nil {
			logrus.Fatal(err)
		}
	}

	// stanConn, err := stan.Connect(
	// 	"nats",
	// 	strings.ReplaceAll(ip, ".", "-"),
	// 	stan.NatsConn(natsConn),
	// 	stan.SetConnectionLostHandler(
	// 		func(_ stan.Conn, reason error) {
	// 			logrus.Fatalf("Connection lost, reason: %v", reason)
	// 		},
	// 	),
	// )
	// if err != nil {
	// 	logrus.Fatalf("can't connect to nats-streaming: %v", err)
	// }

	// manager, err := partitions.NewManager(stanConn, c, logrus.StandardLogger())

	// t := time.Tick(time.Millisecond * 500)
	// i := 0
	// for {
	// 	select {
	// 	case <-t:
	// 		manager.Do(3, []byte(fmt.Sprintf("%s.%d", c.ID(), i)))
	// 		i++
	// 	}
	// }

	_, err = etcdm.Create(etcdm.EtcdManagerConfig{
		EtcdConn:      etcdClient,
		Logger:        logrus.StandardLogger(),
		Cluster:       c,
		NumPartitions: 10,
	})

	if err != nil {
		logrus.Fatal(err)
	}

	select {}
}
