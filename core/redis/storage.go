package redis

import (
	"encoding/json"
	"fmt"

	"github.com/dimuls/camtester/core/entity"
	"github.com/mediocregopher/radix"
)

const taskResultTTL = 24 * 60 * 60

type Storage struct {
	cluster *radix.Cluster
}

func NewStorage(clusterAddrs []string) (*Storage, error) {
	cluster, err := radix.NewCluster(clusterAddrs)
	if err != nil {
		return nil, fmt.Errorf("connect to cluster: %w", err)
	}
	return &Storage{cluster: cluster}, nil
}

func (s *Storage) Close() error {
	return s.cluster.Close()
}

func (s *Storage) TaskResult(taskID string) (tr entity.TaskResult, err error) {
	var trJSON string

	err = s.cluster.Do(radix.Pipeline(
		radix.Cmd(&trJSON, "GET", taskID),
		radix.Cmd(nil, "DEL", taskID)))
	if err != nil {
		err = fmt.Errorf("redis: %w", err)
		return
	}

	if trJSON == "" {
		err = entity.ErrTaskResultNotFound
		return
	}

	err = json.Unmarshal([]byte(trJSON), &tr)
	if err != nil {
		err = fmt.Errorf("JSON unmarshal task result: %w", err)
		return
	}

	return
}

func (s *Storage) AddTaskResult(tr entity.TaskResult) error {
	trJSON, err := json.Marshal(tr)
	if err != nil {
		return fmt.Errorf("JSON marshal task result: %w", err)
	}

	err = s.cluster.Do(radix.FlatCmd(nil, "SET", tr.TaskID,
		string(trJSON), "EX", taskResultTTL))
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}

	return nil
}
