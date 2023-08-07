package easyxporter

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

func InjectFlags(fs *pflag.FlagSet) {
	fs.AddFlagSet(collectorFlags)
}

func Flags() *pflag.FlagSet {
	return collectorFlags
}

type exporterConfig struct {
	Logger        *logrus.Logger
	NameSpace     string
	ListenAddress string
	MetricsPath   string
	MaxRequests   int
	Filter        []string
	Ctx           context.Context
}

type Exporter interface {
	Run() error
}

type Option func(*exporterConfig)

func WithLogger(logger *logrus.Logger) Option {
	return func(ec *exporterConfig) {
		ec.Logger = logger
	}
}

func WithMetricPath(path string) Option {
	return func(ec *exporterConfig) {
		ec.MetricsPath = path
	}
}

func WithMaxRequests(maxRequests int) Option {
	return func(ec *exporterConfig) {
		ec.MaxRequests = maxRequests
	}
}

func WithMetricFilter(filter []string) Option {
	return func(ec *exporterConfig) {
		ec.Filter = filter
	}
}

func WitContext(ctx context.Context) Option {
	return func(ec *exporterConfig) {
		ec.Ctx = ctx
	}
}

// Build Exporter From options
func Build(
	addr string,
	nameSpace string,
	opts ...Option,
) Exporter {
	ec := &exporterConfig{
		ListenAddress: addr,
		NameSpace:     nameSpace,
		MaxRequests:   40,
		MetricsPath:   "/metrics",
		Logger:        logrus.New(),
		Ctx:           context.Background(),
	}

	for _, opt := range opts {
		opt(ec)
	}

	return ec
}

// Run Exporter
func (ec *exporterConfig) Run() error {

	collector, err := newEasyCollector(ec.Logger, ec.NameSpace, ec.Filter...)
	if err != nil {
		return err
	}

	rg := prometheus.NewRegistry()
	rg.MustRegister(collector)

	promHandler := promhttp.HandlerFor(
		rg,
		promhttp.HandlerOpts{
			ErrorLog:            ec.Logger,
			MaxRequestsInFlight: ec.MaxRequests,
		},
	)

	http.Handle(ec.MetricsPath, promHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="` + ec.MetricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	server := &http.Server{Addr: ec.ListenAddress}

	ctx, cancel := context.WithCancel(ec.Ctx)
	var eg errgroup.Group
	eg.Go(func() error {
		defer cancel()
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, unix.SIGINT, unix.SIGTERM)
		<-sigs

		return server.Shutdown(ctx)
	})

	eg.Go(func() error {
		defer cancel()
		return server.ListenAndServe()
	})

	eg.Go(func() error {
		return StartAsyncCollector(ctx, ec.Logger)
	})

	return eg.Wait()
}
