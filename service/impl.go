package service

import (
	context "context"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/kraman/nats-test/lib/consumergroup"
)

type PingImpl struct {
	CG consumergroup.ConsumerGroup
}

func (pi *PingImpl) Ping(ctx context.Context, req *PingRequest) (*empty.Empty, error) {
	pi.CG.Publish("abcd", []byte(req.Msg))
	return &empty.Empty{}, nil
}

var (
	x ServiceServer = &PingImpl{}
)
