package nats

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core/entity"

	stan "github.com/nats-io/stan.go"
)

type TaskResultHandler interface {
	HandleTaskResult(t entity.TaskResult) error
}

type TaskResultConsumer struct {
	taskHandler TaskResultHandler
	conn        stan.Conn
	sub         stan.Subscription
	log         *logrus.Entry
	wg          sync.WaitGroup
}

func NewTaskResultConsumer(natsURL, clusterID, clientID string, concurrency int,
	th TaskResultHandler) (tc *TaskResultConsumer, err error) {

	tc = &TaskResultConsumer{
		taskHandler: th,
		log:         logrus.WithField("subsystem", "nats_task_consumer"),
	}

	var conn stan.Conn

	conn, err = stan.Connect(clusterID, clientID+"-task-result-consumer",
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

	tc.sub, err = tc.conn.Subscribe(
		taskResultsSubject,
		tc.handleMsg,
		stan.SetManualAckMode(),
		stan.MaxInflight(concurrency))
	if err != nil {
		err = fmt.Errorf("subscribe to subject: %w", err)
		return
	}

	return
}

func (tc *TaskResultConsumer) handleMsg(msg *stan.Msg) {
	tc.wg.Add(1)
	go func() {
		defer tc.wg.Done()

		var t entity.TaskResult

		err := json.Unmarshal(msg.Data, &t)
		if err != nil {
			logrus.WithError(err).Error(
				"failed to JSON unmarshal task result: %w", err)
		} else {
			err = tc.taskHandler.HandleTaskResult(t)
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

func (tc *TaskResultConsumer) Close() error {
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
