package main

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/nats"
	"github.com/dimuls/camtester/pinger"
)

func envConfigParam(key, defaultVal string) string {
	if key == "" {
		logrus.Fatal("environment config param with empty key requested")
	}

	v := os.Getenv(key)
	if v == "" {
		v = defaultVal
	}

	if v == "" {
		logrus.WithField("environment_variable", key).
			Fatal("environment config param is empty")
	}

	return v
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	natsURL := envConfigParam("NATS_URL", "")
	natsClusterID := envConfigParam("NATS_CLUSTER_ID", "camtester")
	natsClientID := envConfigParam("NATS_CLIENT_ID", "")
	geoLocation := envConfigParam("GEO_LOCATION", "")
	concurrencyStr := envConfigParam("CONCURRENCY", "100")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse concurrency")
	}

	logrus.Info("environment config params loaded")

	trp, err := nats.NewTaskResultPublisher(natsURL, natsClusterID, natsClientID)
	if err != nil {
		logrus.WithError(err).Fatal(
			"failed create nats task result publisher")
	}
	defer func() {
		err = trp.Close()
		if err != nil {
			logrus.WithError(err).Error(
				"failed to close task result publisher")
		} else {
			logrus.Info("task result publisher stopped")
		}
	}()

	logrus.Info("task result publisher created")

	p := pinger.NewPinger(trp)

	logrus.Info("pinger created")

	tc, err := nats.NewTaskConsumer(natsURL, natsClusterID, natsClientID,
		geoLocation, pinger.TaskType, concurrency, p)
	if err != nil {
		logrus.WithError(err).Fatal("failed create new task consumer")
	}
	defer func() {
		err = tc.Close()
		if err != nil {
			logrus.WithError(err).Error(
				"failed to close task consumer")
		} else {
			logrus.Info("task consumer stopped")
		}
	}()

	logrus.Info("task consumer created and started")

	logrus.Info("camtester-pinger started")

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	logrus.Infof("captured %v signal, stopping", <-signals)

	st := time.Now()

	defer func() {
		logrus.Infof("stopped in %s seconds, exiting", time.Now().Sub(st))
	}()
}
