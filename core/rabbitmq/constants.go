package rabbitmq

import "fmt"

const (
	tasksExchangeKind = "headers"

	tasksResultsRoutingKey = "tasks-results"

	msgTypeHeader      = "Msg-Type"
	taskTypeHeaderName = "Task-Type"

	taskMsgType       = "task"
	taskResultMsgType = "task-result"
)

func tasksRoutingKey(tasksType string) string {
	return fmt.Sprintf("%s-tasks", tasksType)
}
