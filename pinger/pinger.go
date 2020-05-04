package pinger

import (
	"fmt"
	"time"

	"github.com/dimuls/camtester/core/entity"
	"github.com/sirupsen/logrus"
	"github.com/sparrc/go-ping"
)

const TaskType = "ping"

type PingResult struct {
	PacketsSent     int           `json:"packets_sent"`
	PacketsReceived int           `json:"packets_received"`
	MinRtt          time.Duration `json:"min_rtt"`
	MaxRtt          time.Duration `json:"max_rtt"`
	AvgRtt          time.Duration `json:"avg_rtt"`
	StdDevRtt       time.Duration `json:"std_dev_rtt"`
}

type TaskResultPublisher interface {
	PublishTaskResult(entity.TaskResult) error
}

type Pinger struct {
	taskResultPublisher TaskResultPublisher
	log                 *logrus.Entry
}

func NewPinger(trp TaskResultPublisher) *Pinger {
	return &Pinger{
		taskResultPublisher: trp,
		log:                 logrus.WithField("subsystem", "pinger"),
	}
}

func (p *Pinger) HandleTask(t entity.Task) error {
	log := p.log.WithField("task_id", t.ID)

	log.Debug("task recieved")

	var host string

	tr := entity.TaskResult{TaskID: t.ID}

	err := t.UnmarshalPayload(&host)
	if err != nil {
		errMsg := "failed to unmarshal task payload"
		log.WithError(err).WithField("payload", string(t.Payload)).
			Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	pg, err := ping.NewPinger(host)
	if err != nil {
		errMsg := "failed to create pinger"
		log.WithError(err).WithField("host", host).Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	pg.Count = 100
	pg.Interval = 100 * time.Millisecond
	pg.Timeout = 11 * time.Second
	pg.Run()

	stats := pg.Statistics()

	err = tr.MarshalPayload(PingResult{
		PacketsSent:     stats.PacketsSent,
		PacketsReceived: stats.PacketsRecv,
		MinRtt:          stats.MinRtt,
		MaxRtt:          stats.MaxRtt,
		AvgRtt:          stats.AvgRtt,
		StdDevRtt:       stats.StdDevRtt,
	})
	if err != nil {
		log.WithError(err).WithField("payload", stats.PacketLoss).
			Error("failed to marshal task result payload")
		return fmt.Errorf("marshal task result payload: %w", err)
	}

	tr.Ok = true

	err = p.taskResultPublisher.PublishTaskResult(tr)
	if err != nil {
		log.WithError(err).Error("failed to publish task result")
		return fmt.Errorf("publish task result: %w", err)
	}

	log.Debug("task successfully handled")

	return nil
}
func (p *Pinger) handleError(tr entity.TaskResult,
	errMsg string, err error) error {

	err = tr.MarshalPayload(errMsg + ": " + err.Error())
	if err != nil {
		p.log.WithError(err).Error("failed to marshal task result payload")
		return fmt.Errorf("marshal task result payload: %w", err)
	}

	err = p.taskResultPublisher.PublishTaskResult(tr)
	if err != nil {
		p.log.WithError(err).Error("failed to publish task result")
		return fmt.Errorf("publish task result: %w", err)
	}

	return nil
}
