package prober

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"path"

	"github.com/gonum/stat"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core/entity"

	"github.com/dimuls/camtester/prober/ffmpeg"
)

const TaskType = "probe"

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

type Prober struct {
	ffmpegPath, ffprobePath string
	restreamerProvider      RestreamerProvider
	taskResultPublisher     TaskResultPublisher
	log                     *logrus.Entry
}

func NewProber(rp RestreamerProvider, trp TaskResultPublisher,
	ffmpegPath, ffprobePath string) *Prober {
	return &Prober{
		ffmpegPath:          ffmpegPath,
		ffprobePath:         ffprobePath,
		restreamerProvider:  rp,
		taskResultPublisher: trp,
		log:                 logrus.WithField("subsystem", "prober"),
	}
}

func (p *Prober) HandleTask(t entity.Task) error {
	log := p.log.WithField("task_id", t.ID)

	log.Debug("task recieved")

	var uri string

	tr := entity.TaskResult{TaskID: t.ID}

	err := t.UnmarshalPayload(&uri)
	if err != nil {
		errMsg := "failed to unmarshal task payload"
		log.WithError(err).WithField("payload", string(t.Payload)).
			Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	restreamerAddr, err := p.restreamerProvider.ProvideRestreamer(uri)
	if err != nil {
		errMsg := "failed to get restreamer host"
		log.WithError(err).Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	log = log.WithField("restreamer_host", restreamerAddr)

	log.Debug("got restreamer host")

	origURIBase64 := base64.StdEncoding.EncodeToString([]byte(uri))

	uri = fmt.Sprintf("rtsp://%s/%s", restreamerAddr, origURIBase64)

	tempDir := os.TempDir()
	tempFile := path.Join(tempDir, fmt.Sprintf("probe-%d.%s", t.ID,
		sampleContainerExt))

	recordingErrors, err := ffmpeg.RecordStream(p.ffmpegPath, uri, sampleDurationSec,
		tempFile)

	defer func() {
		err = os.Remove(tempFile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.WithError(err).Error("failed to remove sample file")
		}
	}()

	if err != nil {
		errMsg := "failed to record"
		log.WithError(err).Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	vfs, err := ffmpeg.ProbeVideo(p.ffprobePath, tempFile)
	if err != nil {
		errMsg := "failed to probe video"
		log.WithError(err).Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	afs, err := ffmpeg.ProbeAudio(p.ffprobePath, tempFile)
	if err != nil {
		errMsg := "failed to probe audio"
		log.WithError(err).Error(errMsg)
		return p.handleError(tr, errMsg, err)
	}

	var (
		pr ProbeResult

		touts    []float64
		isBlack  bool
		isFreeze bool
	)

	pr.SampleDurationSec = sampleDurationSec
	pr.RecordingErrors = recordingErrors
	pr.VideoFrames = len(vfs)
	pr.AudioFrames = len(afs)

	for i, f := range vfs {
		touts = append(touts, f.Tout)

		if isBlack {
			if f.BlackEnd != nil {
				isBlack = false
			} else {
				pr.BlackFrames++
			}
		} else {
			if f.BlackEnd != nil {
				pr.BlackFrames += i
			}
			if f.BlackStart != nil {
				isBlack = true
				pr.BlackFrames++
			}
		}

		if isFreeze {
			if f.FreezeEnd != nil {
				isFreeze = false
			} else {
				pr.FreezeFrames++
			}
		} else {
			if f.FreezeEnd != nil {
				pr.FreezeFrames += i
			}
			if f.FreezeStart != nil {
				isFreeze = true
				pr.FreezeFrames++
			}
		}

	}

	const lag = 20
	var toutZScores []int

	if len(touts) > lag {
		toutZScores = zScore(touts, lag, 10, 0.5)
	}

	var isPrevTop bool

	for _, zs := range toutZScores {
		if zs == 1 || zs == -1 {
			if !isPrevTop {
				pr.TemporalOutliersPeaks++
				isPrevTop = true
			}
		} else {
			isPrevTop = false
		}
	}

	var isSilence bool

	for i, f := range afs {
		if isSilence {
			if f.SilenceEnd != nil {
				if !isSilence {
					pr.SilenceFrames += i
				}
				isSilence = false
			} else {
				pr.SilenceFrames++
			}
		} else {
			if f.SilenceStart != nil {
				isSilence = true
				pr.SilenceFrames++
			}
		}
	}

	tr.Ok = true

	err = tr.MarshalPayload(pr)
	if err != nil {
		log.WithError(err).Error("failed to marshal task result payload")
		return fmt.Errorf("marshal tast result payload: %w", err)
	}

	err = p.taskResultPublisher.PublishTaskResult(tr)
	if err != nil {
		log.WithError(err).Error("failed to publish task result")
		return fmt.Errorf("publish task result: %w", err)
	}

	log.Debug("task successfully handled")

	return nil
}

func (p *Prober) handleError(tr entity.TaskResult,
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
