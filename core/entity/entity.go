package entity

import (
	"encoding/json"
)

type Task struct {
	ID      int             `json:"id" db:"id"`
	Type    string          `json:"type" db:"type"`
	Payload json.RawMessage `json:"payload" db:"payload"`
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
	ID      int             `json:"id" db:"id"`
	TaskID  int             `json:"task_id" db:"task_id"`
	Ok      bool            `json:"ok" db:"ok"`
	Payload json.RawMessage `json:"payload" db:"payload"`
}

func (t *TaskResult) MarshalPayload(payload interface{}) (err error) {
	t.Payload, err = json.Marshal(payload)
	return
}

func (t TaskResult) UnmarshalPayload(v interface{}) (err error) {
	err = json.Unmarshal(t.Payload, v)
	return
}
