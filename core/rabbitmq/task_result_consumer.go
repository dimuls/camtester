package rabbitmq

import (
	"encoding/json"
	"sync"

	"github.com/assembla/cony"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"

	"github.com/dimuls/camtester/core/entity"
)

type TaskResultHandler interface {
	HandleTaskResult(t entity.TaskResult) error
}

type TaskResultConsumer struct {
	conyClient   *cony.Client
	conyConsumer *cony.Consumer
	wg           sync.WaitGroup
}

func NewTaskResultConsumer(uri, exchange string, concurrency int,
	trh TaskResultHandler) *TaskResultConsumer {

	client := cony.NewClient(cony.URL(uri), cony.Backoff(cony.DefaultBackoff))

	conyExchange := cony.Exchange{
		Name:    exchange,
		Kind:    tasksExchangeKind,
		Durable: true,
	}

	queue := &cony.Queue{
		Name:    tasksResultsRoutingKey,
		Durable: true,
	}

	client.Declare([]cony.Declaration{
		cony.DeclareExchange(conyExchange),
		cony.DeclareQueue(queue),
		cony.DeclareBinding(cony.Binding{
			Queue:    queue,
			Exchange: conyExchange,
			Key:      tasksResultsRoutingKey,
			Args: amqp.Table{
				msgTypeHeader: taskResultMsgType,
			},
		}),
	})

	consumer := cony.NewConsumer(queue, cony.AutoTag(), cony.Qos(concurrency))

	client.Consume(consumer)

	fc := &TaskResultConsumer{
		conyClient:   client,
		conyConsumer: consumer,
	}

	log := logrus.WithFields(logrus.Fields{
		"subsystem": "rabbitmq_task_result_consumer",
	})

	fc.wg.Add(1)
	go func() {
		defer fc.wg.Done()

		for client.Loop() {
			select {

			case d, ok := <-fc.conyConsumer.Deliveries():
				if !ok {
					continue
				}

				var t entity.TaskResult

				err := json.Unmarshal(d.Body, &t)
				if err != nil {
					log.WithError(err).Errorf(
						"failed to JSON unmarshal task")
					continue
				}

				fc.wg.Add(1)
				go func() {
					defer fc.wg.Done()

					err = trh.HandleTaskResult(t)
					if err != nil {
						err = d.Reject(true)
						if err != nil {
							log.WithError(err).Error(
								"failed to reject delivery")
						}
						return
					}

					err = d.Ack(false)
					if err != nil {
						log.WithError(err).Error(
							"failed to acknowledge delivery")
					}
				}()

			case err, ok := <-consumer.Errors():
				if !ok {
					continue
				}
				if err != nil {
					log.WithError(err).Error("got consumer error")
				}

			case err, ok := <-client.Errors():
				if !ok {
					continue
				}
				if err != (*amqp.Error)(nil) {
					log.WithError(err).Error("got client error")
				}
			}
		}
	}()

	return fc
}

func (fc *TaskResultConsumer) Stop() {
	fc.conyConsumer.Cancel()
	fc.conyClient.Close()
	fc.wg.Wait()
}
