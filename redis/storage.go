package redis

import (
	"encoding/json"
	"fmt"

	"github.com/dimuls/camtester/entity"
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

func (s *Storage) Task(taskID string) (t entity.Task, err error) {
	var tJSON string

	err = s.cluster.Do(radix.Cmd(&tJSON, "GET", taskID))
	if err != nil {
		err = fmt.Errorf("redis get: %w", err)
		return
	}

	if tJSON == "" {
		err = entity.ErrTaskNotFound
		return
	}

	err = json.Unmarshal([]byte(tJSON), &t)
	if err != nil {
		err = fmt.Errorf("JSON unmarshal task: %w", err)
		return
	}

	return
}

func (s *Storage) SetTask(t entity.Task) error {
	tJSON, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("JSON marshal task: %w", err)
	}

	err = s.cluster.Do(radix.FlatCmd(nil, "SET", t.ID,
		string(tJSON), "EX", taskResultTTL))
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}
