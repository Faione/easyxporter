package easyxporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestBuildApp(t *testing.T) {
	var config interface{} = Build(":9090", "test", WitContext(context.Background()), WithLogger(logrus.New()), WithMaxRequests(100), WithMetricPath("/test"))
	ec := config.(*exporterConfig)
	fmt.Println(ec)
}
