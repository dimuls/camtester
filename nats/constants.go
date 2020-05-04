package nats

import "fmt"

const (
	taskResultsSubject = "task-results"
)

func tasksSubject(geoLocation string, taskType string) string {
	return fmt.Sprintf("%s.%s.tasks", geoLocation, taskType)
}

func tasksQueueGroup(geoLocation string, taskType string) string {
	return fmt.Sprintf("%s.%s.tasks", geoLocation, taskType)
}
