# easyxporter

a simple and easy framework to make a nodeexporter like exporter by very few codes

three step to write an Exporter

**Step1:**

fill your collect logic by impl `easyxporter.Collector` interface

```go
// hardware_info_linux.go

const (
	hardwareInfoCollectorSubsystem = "hardware"
)

// 通过 sysinfo 获取server的硬件信息
type hardwareInfoCollector struct {
	osInfo      *prometheus.Desc
}

func NewHardwareInfoCollector(logger *logrus.Logger) (easyxporter.Collector, error) {
	return &hardwareInfoCollector{
		osInfo: prometheus.NewDesc(
			prometheus.BuildFQName(easyxporter.GetNameSpace(), hardwareInfoCollectorSubsystem, "os"),
			"OS information from sysinfo",
			[]string{"name", "vendor", "version", "release", "architecture"}, nil,
		),
	}, nil

}

func (h *hardwareInfoCollector) Update(ch chan<- prometheus.Metric) error {
	var si sysinfo.SysInfo
	si.GetSysInfo()

	ch <- prometheus.MustNewConstMetric(
		h.osInfo,
		prometheus.CounterValue,
		1,
		si.OS.Name, si.OS.Vendor, si.OS.Version, si.OS.Release, si.OS.Architecture,
	)

	return nil
}

```

**Step2:**

register your Collector to easyExporter

```go
// hardware_info_linux.go

func init() {
	easyxporter.RegisterCollector(hardwareInfoCollectorSubsystem, true, NewHardwareInfoCollector)
}
```

**Step3:**

run EasyExporter in you main code

```go
easyxporter.Run(easyxporter.ExporterOpts{
		Logger:        logger,
		ListenAddress: listenAddress,
		MetricsPath:   metricsPath,
		MaxRequests:   maxRequests,
		NameSpace:     "server",
	})
```

after alll, you exporter will be started as a metrics server.