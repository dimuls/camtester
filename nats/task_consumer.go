package nats

import (
	"encoding/json"
	"fmt"
	"sync"

	stan "github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/entity"
)

type TaskHandler interface {
	HandleTask(t entity.Task) error
}

type TaskConsumer struct {
	taskHandler TaskHandler
	conn        stan.Conn
	sub         stan.Subscription
	log         *logrus.Entry
	wg          sync.WaitGroup
}

func NewTaskConsumer(natsURL, clusterID, clientID, geoLocation,
	tasksType string, concurrency int, th TaskHandler) (tc *TaskConsumer, err error) {

	tc = &TaskConsumer{
		taskHandler: th,
		log:         logrus.WithField("subsystem", "nats_task_consumer"),
	}

	var conn stan.Conn

	conn, err = stan.Connect(clusterID, clientID+"-task-consumer",
		stan.NatsURL(natsURL))
	if err != nil {
		err = fmt.Errorf("nats connect: %w", err)
		return
	}
	defer func() {
		if err != nil && tc.conn != nil {
			err = tc.conn.Close()
			if err != nil {
				logrus.WithError(err).Error("failed to close connection")
			}
		}
	}()

	tc.conn = conn

	tc.sub, err = tc.conn.QueueSubscribe(
		tasksSubject(geoLocation, tasksType),
		tasksQueueGroup(geoLocation, tasksType),
		tc.handleMsg,
		stan.SetManualAckMode(),
		stan.MaxInflight(concurrency))
	if err != nil {
		err = fmt.Errorf("subscribe to subject: %w", err)
		return
	}

	return
}

func (tc *TaskConsumer) handleMsg(msg *stan.Msg) {
	tc.wg.Add(1)
	go func() {
		defer tc.wg.Done()

		var t entity.Task

		err := json.Unmarshal(msg.Data, &t)
		if err != nil {
			logrus.WithError(err).Error(
				"failed to JSON unmarshal task: %w", err)
		} else {
			err = tc.taskHandler.HandleTask(t)
			if err != nil {
				return
			}
		}

		err = msg.Ack()
		if err != nil {
			tc.log.WithError(err).Error("failed to ack")
		}
	}()
}

func (tc *TaskConsumer) Close() error {
	err := tc.sub.Unsubscribe()
	if err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}

	err = tc.conn.Close()
	if err != nil {
		return fmt.Errorf("close connection: %w", err)
	}

	tc.wg.Wait()

	return nil
}
