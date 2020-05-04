package nats

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/stan.go"

	"github.com/dimuls/camtester/entity"
)

type TaskPublisher struct {
	conn stan.Conn
}

func NewTaskPublisher(natsURL, clusterID, clientID string) (
	tp *TaskPublisher, err error) {

	tp = &TaskPublisher{}
	tp.conn, err = stan.Connect(clusterID, clientID+"-task-publisher",
		stan.NatsURL(natsURL))
	return
}

func (tp *TaskPublisher) Close() error {
	return tp.conn.Close()
}

func (tp *TaskPublisher) PublishTask(t entity.Task) error {
	tJSON, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("JSON marshal task: %w", err)
	}
	return tp.conn.Publish(tasksSubject(t.GeoLocation, t.Type), tJSON)
}
