package nats

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/stan.go"

	"github.com/dimuls/camtester/entity"
)

type TaskResultPublisher struct {
	conn stan.Conn
}

func NewTaskResultPublisher(natsURL, clusterID, clientID string) (
	tp *TaskResultPublisher, err error) {

	tp = &TaskResultPublisher{}
	tp.conn, err = stan.Connect(clusterID, clientID+"-task-result-publisher",
		stan.NatsURL(natsURL))
	return
}

func (tp *TaskResultPublisher) Close() error {
	return tp.conn.Close()
}

func (tp *TaskResultPublisher) PublishTaskResult(t entity.TaskResult) error {
	tJSON, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("JSON marshal task: %w", err)
	}
	return tp.conn.Publish(taskResultsSubject, tJSON)
}
