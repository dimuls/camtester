package main

import (
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core"
	"github.com/dimuls/camtester/core/nats"
	"github.com/dimuls/camtester/core/redis"
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

	bindAddr := envConfigParam("BIND_ADDR", ":80")
	jwtSecret := envConfigParam("JWT_SECRET", "")
	redisClusterAddrsStr := envConfigParam("REDIS_CLUSTER_ADDRS", "")
	natsURL := envConfigParam("NATS_URL", "")
	natsClusterID := envConfigParam("NATS_CLUSTER_ID", "camtester")
	natsClientID := envConfigParam("NATS_CLIENT_ID", "")
	concurrencyStr := envConfigParam("CONCURRENCY", "100")

	redisClusterAddrs := strings.Split(redisClusterAddrsStr, ",")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse concurrency")
	}

	logrus.Info("environment config params loaded")

	dbs, err := redis.NewStorage(redisClusterAddrs)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create redis DB storage")
	}
	defer func() {
		err = dbs.Close()
		if err != nil {
			logrus.WithError(err).Error(
				"failed to close redis DB storage")
		} else {
			logrus.Info("redis DB storage closed")
		}
	}()

	logrus.Info("redis DB storage created")

	tp, err := nats.NewTaskPublisher(natsURL, natsClusterID, natsClientID)
	if err != nil {
		logrus.WithError(err).Fatal(
			"failed to create nats task publisher")
	}
	defer func() {
		err = tp.Close()
		if err != nil {
			logrus.WithError(err).Error(
				"failed to close nats task publisher")
		} else {
			logrus.Info("nats task publisher closed")
		}
	}()

	logrus.Info("nats task publisher created")

	c := core.NewCore(dbs, tp, bindAddr, jwtSecret)
	defer func() {
		err = c.Stop()
		if err != nil {
			logrus.WithError(err).Error("failed to stop core")
		} else {
			logrus.Info("core stopped")
		}
	}()

	logrus.Info("core created and started")

	trc, err := nats.NewTaskResultConsumer(natsURL, natsClusterID, natsClientID,
		concurrency, c)
	if err != nil {
		logrus.WithError(err).Fatal(
			"failed to create nats task result consumer")
	}
	defer func() {
		err = trc.Close()
		if err != nil {
			logrus.WithError(err).Error(
				"failed to close nats task result consumer")
		} else {
			logrus.Info("nats task result consumer closed")
		}
	}()

	logrus.Info("task result consumer created")

	// wait for all goroutines to start
	time.Sleep(200 * time.Millisecond)

	logrus.Info("camtester-core started")

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	logrus.Infof("captured %v signal, stopping", <-signals)

	st := time.Now()

	defer func() {
		logrus.Infof("stopped in %s seconds, exiting", time.Now().Sub(st))
	}()
}
