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

type ExporterOpts struct {
	Logger        *logrus.Logger
	NameSpace     string
	ListenAddress string
	MetricsPath   string
	MaxRequests   int
	Filter        []string
}

func Run(opts ExporterOpts) error {
	collector, err := newEasyCollector(opts.Logger, opts.NameSpace, opts.Filter...)
	if err != nil {
		return err
	}

	rg := prometheus.NewRegistry()
	rg.MustRegister(collector)

	promHandler := promhttp.HandlerFor(
		rg,
		promhttp.HandlerOpts{
			ErrorLog:            opts.Logger,
			MaxRequestsInFlight: opts.MaxRequests,
		},
	)

	http.Handle(opts.MetricsPath, promHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="` + opts.MetricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	server := &http.Server{Addr: opts.ListenAddress}

	ctx, cancel := context.WithCancel(context.Background())
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
		return StartAsyncCollector(ctx, opts.Logger)
	})

	return eg.Wait()
}
