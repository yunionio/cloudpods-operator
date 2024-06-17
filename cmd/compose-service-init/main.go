package main

import (
	"flag"
	"fmt"
	"os"

	"yunion.io/x/log"
	"yunion.io/x/pkg/util/sets"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
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

	ProductVersion string

	publicIp string

	edition string
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

	// productVersion
	flag.StringVar(&ProductVersion, "product-version", string(v1alpha1.ProductVersionCMP), "Product version")

	// edition
	flag.StringVar(&edition, "edition", string(constants.OnecloudCommunityEdition), "Edition")

	flag.StringVar(&publicIp, "public-ip", "127.0.0.1", "Public IP")

	flag.Parse()

	initFlagByEnv()
}

func initFlagByEnv() {
	pIP, ok := os.LookupEnv("PUBLIC_IP")
	if ok && pIP != "" {
		publicIp = pIP
	}
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
	if !sets.NewString(string(v1alpha1.ProductVersionCMP), string(v1alpha1.ProductVersionBaremetal)).Has(ProductVersion) {
		log.Fatalf("unsupported product version: %s", ProductVersion)
	}

	if ProductVersion != string(v1alpha1.ProductVersionCMP) {
		if publicIp == "127.0.0.1" {
			log.Fatalf("public-ip of product version %s shouldn't be %s", ProductVersion, publicIp)
		}
	}

	cluster, err := prepareMan.ConstructCluster(dbConfig, publicIp, targetConfigDir, v1alpha1.ProductVersion(ProductVersion))
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
