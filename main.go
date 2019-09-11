package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/kraman/nats-test/lib/consumergroup"
	etcdm "github.com/kraman/nats-test/lib/consumergroup/etcd"
	ec "github.com/kraman/nats-test/lib/discovery/etcd"
	"github.com/kraman/nats-test/service"
	"google.golang.org/grpc"

	"github.com/coreos/etcd/clientv3"
	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"

	_ "github.com/jackc/pgx"
	_ "github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func main() {
	ip := os.Getenv("POD_IP")

	logrus.StandardLogger().SetLevel(logrus.InfoLevel)

	natsConn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer natsConn.Close()

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://127.0.0.1:3379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		logrus.Fatal(err)
	}
	c, err := ec.Create(ec.EtcdClusterConfig{
		ClusterName: "test",
		NodeName:    ip,
		EtcdConn:    etcdClient,
		Logger:      logrus.StandardLogger(),
		NodeTTLSec:  1 * time.Second,
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

	pingConsumerGroup, err := etcdm.NewConsumerGroup(etcdClient, consumergroup.Config{
		Logger:        logrus.StandardLogger(),
		Subject:       "ping",
		Cluster:       c,
		NumPartitions: 10,
		NodeTTLSec:    time.Second,
		StanConn:      stanConn,
		Handler: func(partitionID string, msg *stan.Msg) {
			logrus.Infof("req on server: %s (%d)", string(msg.Data), msg.Sequence)
			time.Sleep(time.Second)
			logrus.Info("reply on server")
		},
	})
	if err != nil {
		logrus.Fatal(err)
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:1770", ip), grpc.WithInsecure())
	if err != nil {
		logrus.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := service.NewServiceClient(conn)

	time.AfterFunc(time.Second*5, func() {
		for i := 0; i < 20; i++ {
			client.Ping(context.Background(), &service.PingRequest{Msg: fmt.Sprintf("hi %d", i)})
		}
	})

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:1770", ip))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	service.RegisterServiceServer(s, &service.PingImpl{pingConsumerGroup})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
