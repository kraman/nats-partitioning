package main

import (
	"fmt"
	"os"

	"github.com/kraman/nats-test/cmd"
	_ "github.com/kraman/nats-test/lib/discovery/serf"
	_ "github.com/kraman/nats-test/lib/discovery/nats"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
