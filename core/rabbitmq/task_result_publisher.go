package rabbitmq

import (
	"encoding/json"
	"fmt"

	"github.com/assembla/cony"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"

	"github.com/dimuls/camtester/core/entity"
)

type TaskResultPublisher struct {
	client    *cony.Client
	publisher *cony.Publisher
}

func NewTaskResultPublisher(uri, exchange string) *TaskResultPublisher {
	client := cony.NewClient(cony.URL(uri),
		cony.Backoff(cony.DefaultBackoff))

	client.Declare([]cony.Declaration{
		cony.DeclareExchange(cony.Exchange{
			Name:    exchange,
			Kind:    tasksExchangeKind,
			Durable: true,
		}),
	})

	publisher := cony.NewPublisher(exchange, tasksResultsRoutingKey)

	client.Publish(publisher)

	log := logrus.WithFields(logrus.Fields{
		"subsystem": "rabbitmq_task_result_publisher",
	})

	go func() {
		for client.Loop() {
			select {

			case err, ok := <-client.Errors():
				if !ok {
					continue
				}
				if err == (*amqp.Error)(nil) {
					continue
				}
				log.WithError(err).Error("got cony client error")

			case blocked, ok := <-client.Blocking():
				if !ok {
					continue
				}
				log.WithField("reason", blocked.Reason).
					Warn("cony client is blocking")
			}
		}
	}()

	return &TaskResultPublisher{
		client:    client,
		publisher: publisher,
	}
}

func (p *TaskResultPublisher) Stop() {
	p.publisher.Cancel()
	p.client.Close()
}

func (p *TaskResultPublisher) PublishTaskResult(tr entity.TaskResult) error {
	trJSON, err := json.Marshal(tr)
	if err != nil {
		return fmt.Errorf("JSON marshal task result: %w", err)
	}

	return p.publisher.Publish(amqp.Publishing{
		Headers: amqp.Table{
			msgTypeHeader: taskResultMsgType,
		},
		Body: trJSON,
	})
}
