package manager

type MsgHandler func(partitionID string, seqNo uint64, msg []byte)

type ConsumerGroupManager interface {
	Send(partitionKey string, msg []byte) (err error)
}
