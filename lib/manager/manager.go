package manager

import "github.com/kraman/nats-test/lib/cluster"

type PartitionManager interface {
	Invoke(partitionKey string, f func(partitionOwner *cluster.Member))
}
