package main

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core"
	"github.com/dimuls/camtester/core/postgres"
	"github.com/dimuls/camtester/core/rabbitmq"
)

const envPrefix = "CAMTESTER_CORE"

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

	bindAddr := envConfigParam("BIND_ADDR", ":80")
	jwtSecret := envConfigParam("JWT_SECRET", "")
	postgresURI := envConfigParam("POSTGRES_URI", "")
	rabbitMQURI := envConfigParam("RABBITMQ_URI", "")
	rabbitMQExchange := envConfigParam("RABBITMQ_EXCHANGE", "")
	concurrencyStr := envConfigParam("CONCURRENCY", "100")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse concurrency")
	}

	logrus.Info("environment config params loaded")

	dbs, err := postgres.NewStorage(postgresURI)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create postgres DB storage")
	}

	logrus.Info("postgres DB storage created")

	err = dbs.Migrate()
	if err != nil {
		logrus.WithError(err).Fatal("failed to migrate postgres DB storage")
	}

	logrus.Info("postgres DB storage migrated")

	tp := rabbitmq.NewTaskPublisher(rabbitMQURI, rabbitMQExchange)

	logrus.Info("rabbitmq task publisher created and started")

	c := core.NewCore(dbs, tp, bindAddr, jwtSecret)

	logrus.Info("core created and started")

	trc := rabbitmq.NewTaskResultConsumer(rabbitMQURI, rabbitMQExchange,
		concurrency, c)

	logrus.Info("task result consumer created and started")

	// wait for all goroutines to start
	time.Sleep(200 * time.Millisecond)

	logrus.Info("camtester-core started")

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	logrus.Infof("captured %v signal, stopping", <-signals)

	st := time.Now()

	trc.Stop()

	logrus.Info("task result consumer stopped")

	err = c.Stop()
	if err != nil {
		logrus.WithError(err).Error("core stop error")
	}

	logrus.Info("core stopped")

	tp.Stop()

	logrus.Info("task publisher stopped")

	err = dbs.Close()
	if err != nil {
		logrus.WithError(err).Error("postgres DB storage close error")
	}

	logrus.Info("postgres DB storage closed")

	logrus.Infof("stopped in %s seconds, exiting", time.Now().Sub(st))
}
