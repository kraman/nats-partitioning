package es

// import (
// 	"context"
// 	"errors"

// 	"github.com/nats-io/stan.go"
// )

// func GetLastMsg(c stan.Conn, timeoutCtx context.Context) (m *stan.Msg, err error) {
// 	if t, _ := timeoutCtx.Deadline(); t.IsZero() {
// 		return nil, errors.New("no deadline set")
// 	}

// 	lastMessage := make(chan *stan.Msg, 1)
// 	sub, err := c.Subscribe("_PARTITIONS", func(m *stan.Msg) {
// 		lastMessage <- m
// 		m.Ack()
// 	}, stan.StartWithLastReceived(), stan.MaxInflight(1), stan.SetManualAckMode())
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer sub.Close()

// 	select {
// 	case m = <-lastMessage:
// 	case <-timeoutCtx.Done():
// 	}

// 	return m, nil
// }
