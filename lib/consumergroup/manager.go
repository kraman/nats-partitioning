package consumergroup

import (
	"time"

	"github.com/kraman/nats-test/lib/discovery"
	"github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
)

type MsgHandler func(partitionID string, msg *stan.Msg)

type ConsumerGroup interface {
	Publish(partitionKey string, msg []byte) (err error)
	Shutdown() error
}

type Config struct {
	Logger  *logrus.Logger
	Cluster discovery.Cluster
	Subject string

	NumPartitions int
	NodeTTLSec    time.Duration

	StanConn stan.Conn
	Handler  MsgHandler
}
