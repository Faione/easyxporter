package easyxporter

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

var (
	factories       = make(map[string]func(logger *logrus.Logger) (Collector, error))
	asyncCollectors = make(map[string]func(ctx context.Context) error)

	collectorState      = make(map[string]*bool)
	initiatedCollectors = make(map[string]Collector)

	collectorFlags *pflag.FlagSet = &pflag.FlagSet{}
)

// 注册collector
func RegisterCollector(collector string, isDefaultEnabled bool, factory func(logger *logrus.Logger) (Collector, error)) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}

	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
	flag := collectorFlags.Bool(
		flagName,
		isDefaultEnabled,
		flagHelp,
	)

	collectorState[collector] = flag
	factories[collector] = factory
}

// 注册 async collector
func RegisterAsyncCollector(collector string, isDefaultEnabled bool, factory func(logger *logrus.Logger) (AsyncCollector, error)) {
	wrapFactory := func(logger *logrus.Logger) (Collector, error) {
		c, err := factory(logger)
		if err != nil {
			return nil, err
		}

		asyncCollectors[collector] = c.AsyncCollect
		return c, nil
	}
	RegisterCollector(collector, isDefaultEnabled, wrapFactory)
}

// Collector 在每次请求到来时才进行采集并生成metric
type Collector interface {
	// 同步获取metrics并通过prometheus registry暴露
	Update(ch chan<- prometheus.Metric) error
}

// AsyncCollector 在后台采集metric并不断更新，在请求到来时返回最新的metric
type AsyncCollector interface {
	Collector

	// 阻塞并持续在后台进行异步更新
	AsyncCollect(ctx context.Context) error
}

// 开启所有的后台采集器并阻塞直到所有采集器结束
func StartAsyncCollector(ctx context.Context, logger *logrus.Logger) error {

	var eg errgroup.Group
	for name, c := range asyncCollectors {
		collect := c
		eg.Go(func() error {
			return collect(ctx)
		})

		logger.Debug("async collector ", name, " running backend")
	}

	return eg.Wait()
}

// ErrNoData indicates the collector found no data to collect, but had no other error.
var ErrNoData = errors.New("collector returned no data")

func IsNoDataError(err error) bool {
	return err == ErrNoData
}
