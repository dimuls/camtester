package ffmpeg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type VideoFrame struct {
	Tout           float64  `json:"lavfi.signalstats.TOUT"`
	BlackStart     *float64 `json:"lavfi.black_start"`
	BlackEnd       *float64 `json:"lavfi.black_end"`
	FreezeStart    *float64 `json:"lavfi.freezedetect.freeze_start"`
	FreezeEnd      *float64 `json:"lavfi.freezedetect.freeze_end"`
	FreezeDuration *float64 `json:"lavfi.freezedetect.freeze_duration"`
}

type videoStream struct {
	Index int `json:"index"`
}

type videoCheck struct {
	Streams []audioStream `json:"streams"`
}

type videoFrame struct {
	Tags *struct {
		Tout           string  `json:"lavfi.signalstats.TOUT"`
		BlackStart     *string `json:"lavfi.black_start"`
		BlackEnd       *string `json:"lavfi.black_end"`
		FreezeStart    *string `json:"lavfi.freezedetect.freeze_start"`
		FreezeEnd      *string `json:"lavfi.freezedetect.freeze_end"`
		FreezeDuration *string `json:"lavfi.freezedetect.freeze_duration"`
	} `json:"tags"`
}

type videoProbeResult struct {
	Frames []videoFrame `json:"frames"`
}

func ProbeVideo(ffprobePath, fileName string) ([]VideoFrame, error) {

	var (
		out     bytes.Buffer
		errText bytes.Buffer
	)

	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-i", fileName,
		"-show_streams",
		"-select_streams", "v",
		"-print_format",
		"json",
	)

	cmd.Stdout = &out
	cmd.Stderr = &errText

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf(
			"run ffprobe video stream check: %w, error text: %s",
			err, errText.String())
	}

	var check audioCheck

	err = json.NewDecoder(&out).Decode(&check)
	if err != nil {
		return nil, fmt.Errorf("JSON decode check: %w", err)
	}

	if len(check.Streams) == 0 {
		return nil, nil
	}

	cmd = exec.Command(ffprobePath,
		"-v", "error",
		"-f", "lavfi",
		"movie="+fileName+",signalstats=stat=tout,blackdetect,freezedetect",
		"-show_frames",
		"-print_format",
		"json",
	)

	out.Reset()
	errText.Reset()

	cmd.Stdout = &out
	cmd.Stderr = &errText

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("run ffprobe: %w, error text: %s",
			err, errText.String())
	}

	var res videoProbeResult

	err = json.NewDecoder(&out).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("JSON decode result: %w", err)
	}

	var vfs []VideoFrame

	for _, f := range res.Frames {
		var vf VideoFrame

		if f.Tags != nil {

			vf.Tout, err = strconv.ParseFloat(f.Tags.Tout, 64)
			if err != nil {
				return nil, fmt.Errorf("parse Tout: %w", err)
			}

			if vf.BlackStart != nil {
				bs, err := strconv.ParseFloat(*f.Tags.BlackStart, 64)
				if err != nil {
					return nil, fmt.Errorf("parse BlackStart: %w", err)
				}
				vf.BlackStart = &bs
			}

			if vf.BlackEnd != nil {
				be, err := strconv.ParseFloat(*f.Tags.BlackEnd, 64)
				if err != nil {
					return nil, fmt.Errorf("parse BlackEnd: %w", err)
				}
				vf.BlackEnd = &be
			}

			if vf.FreezeStart != nil {
				fs, err := strconv.ParseFloat(*f.Tags.FreezeStart, 64)
				if err != nil {
					return nil, fmt.Errorf("parse FreezeStart: %w", err)
				}
				vf.FreezeStart = &fs
			}

			if vf.FreezeEnd != nil {
				fe, err := strconv.ParseFloat(*f.Tags.FreezeEnd, 64)
				if err != nil {
					return nil, fmt.Errorf("parse FreezeEnd: %w", err)
				}
				vf.FreezeEnd = &fe
			}

			if vf.FreezeDuration != nil {
				fd, err := strconv.ParseFloat(*f.Tags.FreezeDuration, 64)
				if err != nil {
					return nil, fmt.Errorf("parse FreezeDuration: %w", err)
				}
				vf.FreezeDuration = &fd
			}
		}

		vfs = append(vfs, vf)
	}

	return vfs, nil
}

type AudioFrame struct {
	SilenceStart    *float64 `json:"lavfi.silence_start"`
	SilenceEnd      *float64 `json:"lavfi.silence_end"`
	SilenceDuration *float64 `json:"lavfi.silence_duration"`
}

type audioStream struct {
	Index int `json:"index"`
}

type audioCheck struct {
	Streams []audioStream `json:"streams"`
}

type audioFrame struct {
	Tags *struct {
		SilenceStart    *string `json:"lavfi.silence_start"`
		SilenceEnd      *string `json:"lavfi.silence_end"`
		SilenceDuration *string `json:"lavfi.silence_duration"`
	} `json:"tags"`
}

type audioProbeResult struct {
	Frames []audioFrame `json:"frames"`
}

func ProbeAudio(ffprobePath, fileName string) ([]AudioFrame, error) {
	var (
		out     bytes.Buffer
		errText bytes.Buffer
	)

	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-i", fileName,
		"-show_streams",
		"-select_streams", "a",
		"-print_format",
		"json",
	)

	cmd.Stdout = &out
	cmd.Stderr = &errText

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf(
			"run ffprobe audio stream check: %w, error text: %s",
			err, errText.String())
	}

	var check audioCheck

	err = json.NewDecoder(&out).Decode(&check)
	if err != nil {
		return nil, fmt.Errorf("JSON decode check: %w", err)
	}

	if len(check.Streams) == 0 {
		return nil, nil
	}

	cmd = exec.Command(ffprobePath,
		"-v", "error",
		"-f", "lavfi",
		"amovie="+fileName+",silencedetect",
		"-show_frames",
		"-print_format",
		"json",
	)

	out.Reset()
	errText.Reset()

	cmd.Stdout = &out
	cmd.Stderr = &errText

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("run ffprobe: %w, error text: %s",
			err, errText.String())
	}

	var res audioProbeResult

	err = json.NewDecoder(&out).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("JSON decode result: %w", err)
	}

	var afs []AudioFrame

	for _, f := range res.Frames {
		var af AudioFrame

		if f.Tags != nil {
			if f.Tags.SilenceStart != nil {
				ss, err := strconv.ParseFloat(*f.Tags.SilenceStart, 64)
				if err != nil {
					return nil, fmt.Errorf("parse SilenceStart: %w", err)
				}
				af.SilenceStart = &ss
			}

			if f.Tags.SilenceEnd != nil {
				se, err := strconv.ParseFloat(*f.Tags.SilenceEnd, 64)
				if err != nil {
					return nil, fmt.Errorf("parse SilenceEnd: %w", err)
				}
				af.SilenceEnd = &se
			}

			if f.Tags.SilenceDuration != nil {
				sd, err := strconv.ParseFloat(*f.Tags.SilenceDuration, 64)
				if err != nil {
					return nil, fmt.Errorf("parse SilenceDuration: %w", err)
				}
				af.SilenceDuration = &sd
			}
		}

		afs = append(afs, af)
	}

	return afs, nil
}
