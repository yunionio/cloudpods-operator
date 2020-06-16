package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
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
			"host_ip": nodeIp,
		},
	}
	conf := getConfig(kwargs)
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

func getConfig(kwargs map[string]interface{}) string {
	conf := ""
	conf += "[global_tags]\n"
	if tags, ok := kwargs["tags"]; ok {
		tgs, _ := tags.(map[string]string)
		for k, v := range tgs {
			conf += fmt.Sprintf("  %s = \"%s\"\n", k, v)
		}
	}
	conf += "\n"
	conf += "[agent]\n"
	conf += "  interval = \"60s\"\n"
	conf += "  round_interval = true\n"
	conf += "  metric_batch_size = 1000\n"
	conf += "  metric_buffer_limit = 10000\n"
	conf += "  collection_jitter = \"0s\"\n"
	conf += "  flush_interval = \"60s\"\n"
	conf += "  flush_jitter = \"0s\"\n"
	conf += "  precision = \"\"\n"
	conf += "  debug = false\n"
	conf += "  quiet = false\n"
	conf += "  logfile = \"/var/log/telegraf/telegraf.err.log\"\n"
	var hostname string
	if hn, ok := kwargs["hostname"]; ok {
		hostname, _ = hn.(string)
	}
	conf += fmt.Sprintf("  hostname = \"%s\"\n", hostname)
	conf += "  omit_hostname = false\n"
	conf += "\n"
	if ifluxb, ok := kwargs["influxdb"]; ok {
		influxdb, _ := ifluxb.(map[string]interface{})
		inUrls, _ := influxdb["url"]
		tUrls, _ := inUrls.([]string)
		inDatabase, _ := influxdb["database"]
		tdb, _ := inDatabase.(string)
		urls := []string{}
		for _, u := range tUrls {
			urls = append(urls, fmt.Sprintf("\"%s\"", u))
		}
		conf += "[[outputs.influxdb]]\n"
		conf += fmt.Sprintf("  urls = [%s]\n", strings.Join(urls, ", "))
		conf += fmt.Sprintf("  database = \"%s\"\n", tdb)
		conf += "  insecure_skip_verify = true\n"
		conf += "\n"
	}
	if kafka, ok := kwargs["kafka"]; ok {
		ka, _ := kafka.(map[string]interface{})
		bks, _ := ka["brokers"]
		tbk, _ := bks.([]string)
		brokers := []string{}
		for _, b := range tbk {
			brokers = append(brokers, fmt.Sprintf("\"%s\"", b[len("kafka://\n"):]))
		}
		conf += "[[outputs.kafka]]\n"
		conf += fmt.Sprintf("  brokers = [%s]\n", strings.Join(brokers, ", "))

		topic, _ := ka["topic"]
		itopic, _ := topic.(string)
		conf += fmt.Sprintf("  topic = \"%s\"\n", itopic)
		conf += "  compression_codec = 0\n"
		conf += "  required_acks = -1\n"
		conf += "  max_retry = 3\n"
		conf += "  data_format = \"json\"\n"
		conf += "  json_timestamp_units = \"1ms\"\n"
		conf += "  routing_tag = \"host\"\n"
		conf += "\n"
	}
	conf += "[[inputs.cpu]]\n"
	conf += "  percpu = false\n"
	conf += "  totalcpu = true\n"
	conf += "  collect_cpu_time = false\n"
	conf += "  report_active = true\n"
	conf += "\n"
	conf += "[[inputs.disk]]\n"
	conf += "  ignore_fs = [\"tmpfs\", \"devtmpfs\", \"overlay\", \"squashfs\", \"iso9660\"]\n"
	conf += "\n"
	conf += "[[inputs.diskio]]\n"
	conf += "  skip_serial_number = false\n"
	conf += "  excludes = \"^nbd\"\n"
	conf += "\n"
	conf += "[[inputs.kernel]]\n"
	conf += "\n"
	conf += "[[inputs.kernel_vmstat]]\n"
	conf += "\n"
	conf += "[[inputs.mem]]\n"
	conf += "\n"
	conf += "[[inputs.processes]]\n"
	conf += "\n"
	conf += "[[inputs.swap]]\n"
	conf += "\n"
	conf += "[[inputs.system]]\n"
	conf += "\n"
	conf += "[[inputs.net]]\n"
	if nics, ok := kwargs["nics"]; ok {
		ns, _ := nics.([]map[string]interface{})
		infs := []string{}
		for _, n := range ns {
			iname, _ := n["name"]
			name, _ := iname.(string)
			infs = append(infs, fmt.Sprintf("\"%s\"", name))
		}
		conf += fmt.Sprintf("  interfaces = [%s]\n", strings.Join(infs, ", "))
		conf += "\n"
		for _, n := range ns {
			iname, _ := n["name"]
			name, _ := iname.(string)
			ialias, _ := n["alias"]
			alias, _ := ialias.(string)
			ispeed, _ := n["speed"]
			speed, _ := ispeed.(int)

			conf += "  [[inputs.net.interface_conf]]\n"
			conf += fmt.Sprintf("    name = \"%s\"\n", name)
			conf += fmt.Sprintf("    alias = \"%s\"\n", alias)
			conf += fmt.Sprintf("    speed = %d\n", speed)
			conf += "\n"
		}
	}
	conf += "[[inputs.netstat]]\n"
	conf += "\n"
	conf += "[[inputs.nstat]]\n"
	conf += "\n"
	conf += "[[inputs.ntpq]]\n"
	conf += "  dns_lookup = false\n"
	conf += "\n"
	if pidFile, ok := kwargs["pid_file"]; ok {
		pf, _ := pidFile.(string)
		conf += "[[inputs.procstat]]\n"
		conf += fmt.Sprintf("  pid_file = \"%s\"\n", pf)
		conf += "\n"
	}
	conf += "[[inputs.internal]]\n"
	conf += "  collect_memstats = false\n"
	conf += "\n"
	conf += "[[inputs.http_listener]]\n"
	conf += "  service_address = \"localhost:8087\"\n"
	conf += "\n"
	return conf
}
