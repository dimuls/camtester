package rabbitmq

import (
	"encoding/json"
	"sync"

	"github.com/assembla/cony"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"

	"github.com/dimuls/camtester/core/entity"
)

type TaskHandler interface {
	HandleTask(t entity.Task) error
}

type TaskConsumer struct {
	conyClient   *cony.Client
	conyConsumer *cony.Consumer
	wg           sync.WaitGroup
}

func NewTaskConsumer(uri, exchange, tasksType string, concurrency int,
	th TaskHandler) *TaskConsumer {

	client := cony.NewClient(cony.URL(uri), cony.Backoff(cony.DefaultBackoff))

	conyExchange := cony.Exchange{
		Name:    exchange,
		Kind:    tasksExchangeKind,
		Durable: true,
	}

	routingKey := tasksRoutingKey(tasksType)

	queue := &cony.Queue{
		Name:    routingKey,
		Durable: true,
	}

	client.Declare([]cony.Declaration{
		cony.DeclareExchange(conyExchange),
		cony.DeclareQueue(queue),
		cony.DeclareBinding(cony.Binding{
			Queue:    queue,
			Exchange: conyExchange,
			Key:      routingKey,
			Args: amqp.Table{
				msgTypeHeader:      taskMsgType,
				taskTypeHeaderName: tasksType,
			},
		}),
	})

	consumer := cony.NewConsumer(queue, cony.AutoTag(), cony.Qos(concurrency))

	client.Consume(consumer)

	fc := &TaskConsumer{
		conyClient:   client,
		conyConsumer: consumer,
	}

	log := logrus.WithFields(logrus.Fields{
		"subsystem": "rabbitmq_task_consumer",
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

				var t entity.Task

				err := json.Unmarshal(d.Body, &t)
				if err != nil {
					log.WithError(err).Errorf(
						"failed to JSON unmarshal task")
					continue
				}

				fc.wg.Add(1)
				go func() {
					defer fc.wg.Done()

					err = th.HandleTask(t)
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

func (fc *TaskConsumer) Stop() {
	fc.conyConsumer.Cancel()
	fc.conyClient.Close()
	fc.wg.Wait()
}
