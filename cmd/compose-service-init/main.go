package main

import (
	"flag"
	"fmt"
	"os"

	"yunion.io/x/log"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/service-init/component"
	"yunion.io/x/onecloud-operator/pkg/version"
)

var (
	printVersion bool

	// mysql options
	mysqlHost     string
	mysqlPort     uint
	mysqlUser     string
	mysqlPassword string

	// target configuration directory
	targetConfigDir string

	step string

	Component string
)

func init() {
	flag.BoolVar(&printVersion, "V", false, "Show version and quit")
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")

	// mysql options
	flag.StringVar(&mysqlHost, "mysql-host", "", "Mysql host")
	flag.UintVar(&mysqlPort, "mysql-port", 3306, "Mysql port")
	flag.StringVar(&mysqlUser, "mysql-user", "root", "Mysql user")
	flag.StringVar(&mysqlPassword, "mysql-password", "", "Mysql user password")

	// target configuration options
	flag.StringVar(&targetConfigDir, "config-dir", ".", "Component configuration directory")

	// run step
	flag.StringVar(&step, "step", "init", "Step to run, choose from ['init', 'post-init']")

	// component
	flag.StringVar(&Component, "component", "", "Service component to init")

	flag.Parse()
}

func main() {
	if printVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	dbConfig := v1alpha1.Mysql{
		Host:     mysqlHost,
		Port:     int32(mysqlPort),
		Username: mysqlUser,
		Password: mysqlPassword,
	}
	prepareMan := component.NewPrepareManager()
	cluster, err := prepareMan.ConstructCluster(dbConfig, targetConfigDir)
	if err != nil {
		log.Fatalf("construct cluster error: %v", err)
	}
	clusterCfg, err := prepareMan.ConstructClusterConfig(targetConfigDir)
	if err != nil {
		log.Fatalf("construct cluster config error: %v", err)
	}

	svcComp := component.GetComponent(v1alpha1.ComponentType(Component))
	if svcComp == nil {
		log.Fatalf("not found service component %q", Component)
	}

	if step == "init" {
		if err := prepareMan.Init(svcComp, cluster, clusterCfg, targetConfigDir); err != nil {
			log.Fatalf("init service: %v", err)
		}
	} else if step == "post-init" {
		if err := prepareMan.PostInit(svcComp, cluster, targetConfigDir); err != nil {
			log.Fatalf("post-init service: %v", err)
		}
	} else {
		log.Fatalf("unknown step %q", step)
	}
}
