package onecloud

const (
	DevtoolTelegrafConf = `
[global_tags]
    platform = "{{ server_hypervisor }}"
    zone_id = "{{ server_zone_id }}"
    cloudregion_id = "{{ server_region_id }}"
    host_ip = "{{ server_ip }}"
    host_id = "{{ server_id }}"

[agent]
    interval = "60s"
    round_interval = true
    metric_batch_size = 1000
    metric_buffer_limit = 10000
    collection_jitter = "0s"
    flush_interval = "60s"
    flush_jitter = "0s"
    precision = ""
    debug = false
    quiet = false
    logfile = "/var/log/telegraf/telegraf.err.log"
    hostname = "{{ server_name }}"
    omit_hostname = false

[[outputs.influxdb]]
    urls = ["{{ influxdb }}"]
    database = "telegraf"
    insecure_skip_verify = true

[[inputs.cpu]]
    percpu = false
    totalcpu = true
    collect_cpu_time = false
    report_active = true

[[inputs.disk]]
    ignore_fs = ["tmpfs", "devtmpfs", "overlay", "squashfs", "iso9660"]

[[inputs.diskio]]
    skip_serial_number = false
    excludes = "^nbd"

[[inputs.kernel]]

[[inputs.kernel_vmstat]]

[[inputs.mem]]

[[inputs.processes]]

[[inputs.swap]]

[[inputs.system]]

[[inputs.net]]

[[inputs.netstat]]

[[inputs.nstat]]

[[inputs.ntpq]]
    dns_lookup = false

[[inputs.internal]]
    collect_memstats = false

[[inputs.http_listener]]
    service_address = "localhost:8087"
`
)
