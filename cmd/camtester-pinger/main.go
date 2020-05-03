package main

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core/rabbitmq"
	"github.com/dimuls/camtester/pinger"
)

const envPrefix = "CAMTESTER_PINGER"

func envConfigParam(key, defaultVal string) string {
	if key == "" {
		logrus.Fatal("environment config param with empty key requested")
	}

	key = envPrefix + "_" + key

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

	rabbitMQURI := envConfigParam("RABBITMQ_URI", "")
	rabbitMQExchange := envConfigParam("RABBITMQ_EXCHANGE", "")
	concurrencyStr := envConfigParam("CONCURRENCY", "100")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse concurrency")
	}

	logrus.Info("environment config params loaded")

	trp := rabbitmq.NewTaskResultPublisher(rabbitMQURI, rabbitMQExchange)

	logrus.Info("task result publisher created and started")

	p := pinger.NewPinger(trp)

	logrus.Info("pinger created")

	tc := rabbitmq.NewTaskConsumer(rabbitMQURI, rabbitMQExchange,
		pinger.TaskType, concurrency, p)

	logrus.Info("task consumer created and started")

	logrus.Info("camtester-pinger started")

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	logrus.Infof("captured %v signal, stopping", <-signals)

	st := time.Now()

	tc.Stop()

	logrus.Info("task consumer stopped")

	trp.Stop()

	logrus.Info("task result publisher stopped")

	logrus.Infof("stopped in %s seconds, exiting", time.Now().Sub(st))
}
