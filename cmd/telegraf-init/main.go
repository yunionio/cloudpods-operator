package main

import (
	"context"
	"log"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"yunion.io/x/onecloud/pkg/hostman/hostinfo/hostconsts"
	"yunion.io/x/onecloud/pkg/hostman/system_service"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	nodeName := os.Getenv("NODENAME")
	if len(nodeName) == 0 {
		log.Fatalf("Missing env nodename")
	}
	influxAddr := os.Getenv("INFLUXDB_URL")
	if len(influxAddr) == 0 {
		log.Fatalf("Missing influxdb url")
	}
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	var masterAddress string
	if length := len(node.Status.Conditions); length > 0 {
		if node.Status.Conditions[length-1].Type == v1.NodeReady &&
			node.Status.Conditions[length-1].Status == v1.ConditionTrue {
			for _, addr := range node.Status.Addresses {
				if addr.Type == v1.NodeInternalIP {
					masterAddress = addr.Address
					break
				}
			}
		}
	}

	if node.Labels[constants.OnecloudEnableHostLabelKey] == "enable" {
		// host enabled
		checkTelegrafConfigExist()
	} else {
		// host disabled
		generateTelegrafConfig(nodeName, masterAddress, influxAddr)
	}
	if err := os.MkdirAll(TELEGRAF_DIR, 775); err != nil {
		log.Fatal(err)
	}
}

const TELEGRAF_CONF = "/etc/telegraf/telegraf.conf"
const TELEGRAF_DIR = "/etc/telegraf/telegraf.d"

func checkTelegrafConfigExist() {
	if _, err := os.Stat(TELEGRAF_CONF); err != nil {
		log.Fatalf("stat telegraf config file error %s", err)
	}
}

func generateTelegrafConfig(nodeName, nodeIp, influxAddr string) {
	kwargs := map[string]interface{}{
		"influxdb": map[string]interface{}{
			"database": "telegraf",
			"url":      []string{influxAddr},
		},
		"hostname": nodeName,
		"tags": map[string]string{
			"host_ip":                             nodeIp,
			hostconsts.TELEGRAF_TAG_KEY_BRAND:     hostconsts.TELEGRAF_TAG_ONECLOUD_BRAND,
			hostconsts.TELEGRAF_TAG_KEY_RES_TYPE:  hostconsts.TELEGRAF_TAG_ONECLOUD_RES_TYPE,
			hostconsts.TELEGRAF_TAG_KEY_HOST_TYPE: hostconsts.TELEGRAF_TAG_ONECLOUD_HOST_TYPE_HOST,
		},
	}
	telegraf := system_service.NewTelegrafService()
	conf := telegraf.GetConfig(kwargs)
	var mode = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	fd, err := os.OpenFile(TELEGRAF_CONF, mode, 0644)
	if err != nil {
		log.Fatalf("open telegraf conf %s", err)
	}
	defer fd.Close()
	_, err = fd.WriteString(conf)
	if err != nil {
		log.Fatalf("write telegraf conf %s", err)
	}
}
