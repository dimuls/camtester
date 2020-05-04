package entity

import (
	"encoding/json"
	"errors"
)

type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	GeoLocation string          `json:"geo_location"`
	Payload     json.RawMessage `json:"payload"`
}

func (t *Task) MarshalPayload(payload interface{}) (err error) {
	t.Payload, err = json.Marshal(payload)
	return
}

func (t Task) UnmarshalPayload(v interface{}) (err error) {
	err = json.Unmarshal(t.Payload, v)
	return
}

type TaskResult struct {
	TaskID  string          `json:"task_id"`
	Ok      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload"`
}

func (t *TaskResult) MarshalPayload(payload interface{}) (err error) {
	t.Payload, err = json.Marshal(payload)
	return
}

func (t TaskResult) UnmarshalPayload(v interface{}) (err error) {
	err = json.Unmarshal(t.Payload, v)
	return
}

var (
	ErrTaskResultNotFound = errors.New("task result not found")
)
