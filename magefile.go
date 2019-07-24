// +build mage

package main

import (
	// mage:import
	k "github.com/kraman/mage-helpers/kubernetes"
	// mage:import
	// _ "github.com/kraman/mage-helpers/protobuf"
	_ "github.com/kraman/mage-helpers/config"
	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func Build() error {
	mg.Deps(k.LoadKindConfig)
	clusterName := viper.GetString("cluster_name")

	if err := sh.RunWith(
		map[string]string{
			"CGO_ENABLED": "0",
			"GOOS": "linux",
			"GOARCH": "amd64",
		},
		"go", "build", "-o", "server", "main.go",
	); err != nil {
		return err
	}
	if err := sh.RunV("docker", "build", "-t", "server:latest", "."); err != nil {
		return err
	}
	if err := sh.RunV("kind", "load", "docker-image", "server:latest", "--name", clusterName); err != nil {
		return err
	}
	if err := sh.RunV("kubectl", "delete", "-f", "deployment.yaml"); err != nil {
		logrus.Info(err)
	}
	return sh.RunV("kubectl", "apply", "-f", "deployment.yaml")
}