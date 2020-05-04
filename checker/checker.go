package checker

import (
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"github.com/gonum/stat"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/entity"
	"github.com/dimuls/camtester/ffmpeg"
)

const TaskType = "check"

const sampleContainerExt = "mkv"
const sampleDurationSec = 10

type ProbeResult struct {
	SampleDurationSec     int `json:"sample_duration_sec"`
	RecordingErrors       int `json:"recording_errors"`
	VideoFrames           int `json:"video_frames"`
	BlackFrames           int `json:"black_frames"`
	FreezeFrames          int `json:"freeze_frames"`
	TemporalOutliersPeaks int `json:"temporal_outliers_peaks"`
	AudioFrames           int `json:"audio_frames"`
	SilenceFrames         int `json:"silence_frames"`
}

type RestreamerProvider interface {
	ProvideRestreamer(uri string) (string, error)
}

type TaskResultPublisher interface {
	PublishTaskResult(entity.TaskResult) error
}

type Checker struct {
	ffmpegPath          string
	restreamerProvider  RestreamerProvider
	taskResultPublisher TaskResultPublisher
	log                 *logrus.Entry
}

func NewChecker(rp RestreamerProvider, trp TaskResultPublisher,
	ffmpegPath string) *Checker {
	return &Checker{
		ffmpegPath:          ffmpegPath,
		restreamerProvider:  rp,
		taskResultPublisher: trp,
		log:                 logrus.WithField("subsystem", "checker"),
	}
}

func (c *Checker) HandleTask(t entity.Task) error {
	log := c.log.WithField("task_id", t.ID)

	log.Debug("task received")

	var uri string

	tr := entity.TaskResult{TaskID: t.ID}

	err := t.UnmarshalPayload(&uri)
	if err != nil {
		errMsg := "failed to unmarshal task payload"
		log.WithError(err).WithField("payload", string(t.Payload)).
			Error(errMsg)
		return c.handleError(tr, errMsg, err)
	}

	restreamerAddr, err := c.restreamerProvider.ProvideRestreamer(uri)
	if err != nil {
		errMsg := "failed to get restreamer host"
		log.WithError(err).Error(errMsg)
		return c.handleError(tr, errMsg, err)
	}

	log = log.WithField("restreamer_host", restreamerAddr)

	log.Debug("got restreamer host")

	origURIBase64 := base64.StdEncoding.EncodeToString([]byte(uri))

	uri = fmt.Sprintf("rtsp://%s/%s", restreamerAddr, origURIBase64)

	ch, err := ffmpeg.CheckStream(c.ffmpegPath, uri, sampleDurationSec)
	if err != nil {
		errMsg := "failed to check stream"
		log.WithError(err).Error(errMsg)
		return c.handleError(tr, errMsg, err)
	}

	tr.Ok = true
	tr.Time = time.Now()

	err = tr.MarshalPayload(ch)
	if err != nil {
		log.WithError(err).Error("failed to marshal task result payload")
		return fmt.Errorf("marshal tast result payload: %w", err)
	}

	err = c.taskResultPublisher.PublishTaskResult(tr)
	if err != nil {
		log.WithError(err).Error("failed to publish task result")
		return fmt.Errorf("publish task result: %w", err)
	}

	log.Debug("task successfully handled")

	return nil
}

func (c *Checker) handleError(tr entity.TaskResult,
	errMsg string, err error) error {

	tr.Time = time.Now()

	err = tr.MarshalPayload(errMsg + ": " + err.Error())
	if err != nil {
		c.log.WithError(err).Error("failed to marshal task result payload")
		return fmt.Errorf("marshal task result payload: %w", err)
	}

	err = c.taskResultPublisher.PublishTaskResult(tr)
	if err != nil {
		c.log.WithError(err).Error("failed to publish task result")
		return fmt.Errorf("publish task result: %w", err)
	}

	return nil
}

func zScore(samples []float64, lag int, threshold, influence float64) (signals []int) {
	signals = make([]int, len(samples))
	filteredY := make([]float64, len(samples))
	for i, sample := range samples[0:lag] {
		filteredY[i] = sample
	}
	avgFilter := make([]float64, len(samples))
	stdFilter := make([]float64, len(samples))

	avgFilter[lag], stdFilter[lag] = stat.MeanStdDev(samples[0:lag], nil)

	for i := lag + 1; i < len(samples); i++ {

		f := float64(samples[i])

		if float64(math.Abs(samples[i]-avgFilter[i-1])) > threshold*float64(stdFilter[i-1]) {
			if samples[i] > avgFilter[i-1] {
				signals[i] = 1
			} else {
				signals[i] = -1
			}
			filteredY[i] = float64(influence*f + (1-influence)*float64(filteredY[i-1]))
			avgFilter[i], stdFilter[i] = stat.MeanStdDev(filteredY[(i-lag):i], nil)
		} else {
			signals[i] = 0
			filteredY[i] = samples[i]
			avgFilter[i], stdFilter[i] = stat.MeanStdDev(filteredY[(i-lag):i], nil)
		}
	}

	return
}
