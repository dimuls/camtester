package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const ComplextTaskType = "complex"

type Task struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	GeoLocation string `json:"geo_location"`

	Payload json.RawMessage `json:"payload,omitempty"`
	Result  *TaskResult     `json:"result,omitempty"`

	Payloads []Task       `json:"payloads,omitempty"`
	Results  []TaskResult `json:"results,omitempty"`
}

func (t Task) Validate() error {
	if t.Type == "" {
		return errors.New("type is empty")
	}

	if t.GeoLocation == "" {
		return errors.New("geo_location is empty")
	}

	if t.Type == ComplextTaskType {
		if len(t.Payloads) == 0 {
			return errors.New("payloads is empty")
		}
		for i, st := range t.Payloads {
			if st.Type == "" {
				return fmt.Errorf("subtask #%d: type is empty", i)
			}
			if st.Type == ComplextTaskType {
				return fmt.Errorf("subtask #%d is complex", i)
			}
			if len(st.Payload) == 0 {
				return fmt.Errorf("subtask #%d: payload is empty", i)
			}
		}

	} else {
		if len(t.Payload) == 0 {
			return errors.New("payload is empty")
		}
	}
	return nil
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
	TaskID  string          `json:"task_id,omitempty"`
	Time    time.Time       `json:"time"`
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
	ErrTaskNotFound = errors.New("task not found")
)
