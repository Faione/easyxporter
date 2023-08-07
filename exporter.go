package easyxporter

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type EasyCollector struct {
	logger             *logrus.Logger
	Collectors         map[string]Collector
	scrapeDurationDesc *prometheus.Desc
	scrapeSuccessDesc  *prometheus.Desc
}

func newEasyCollector(logger *logrus.Logger, namespace string, filters ...string) (*EasyCollector, error) {
	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !*enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}

	collectors := make(map[string]Collector)

	for key, enabled := range collectorState {
		if !*enabled || (len(f) > 0 && !f[key]) {
			continue
		}
		if collector, ok := initiatedCollectors[key]; ok {
			collectors[key] = collector
		} else {
			logger.Debug("init collector: ", key)

			collector, err := factories[key](logger)
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
			initiatedCollectors[key] = collector

		}

	}

	return &EasyCollector{
		logger:     logger,
		Collectors: collectors,
		scrapeDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
			"node_exporter: Duration of a collector scrape.",
			[]string{"collector"},
			nil,
		),
		scrapeSuccessDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "scrape", "collector_success"),
			"node_exporter: Whether a collector succeeded.",
			[]string{"collector"},
			nil,
		),
	}, nil
}

func (s EasyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- s.scrapeDurationDesc
	ch <- s.scrapeSuccessDesc
}

func (s EasyCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(s.Collectors))

	// collect from sync collectors
	for name, c := range s.Collectors {
		go func(name string, c Collector) {
			s.execute(name, c, ch, s.logger)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func (s EasyCollector) execute(name string, c Collector, ch chan<- prometheus.Metric, logger *logrus.Logger) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if IsNoDataError(err) {
			logger.Debug(
				"msg: ", "collector returned no data",
				" name: ", name,
				" duration_seconds: ", duration.Seconds(),
				" err: ", err,
			)
		} else {
			logger.Error(
				"msg: ", "collector failed",
				" name: ", name,
				" duration_seconds: ", duration.Seconds(),
				" err: ", err,
			)
		}
		success = 0
	} else {
		logger.Debug(
			"msg: ", "collector succeeded",
			" name: ", name,
			" duration_seconds: ", duration.Seconds(),
		)
		success = 1
	}

	ch <- prometheus.MustNewConstMetric(s.scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(s.scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}
