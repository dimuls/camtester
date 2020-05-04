package ffmpeg

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func RecordStream(ffmpegPath, uri string, durationSec int, destFile string) (int, error) {
	cmd := exec.Command(ffmpegPath, "-v", "error", "-y", "-i", uri,
		"-t", strconv.Itoa(durationSec), "-c:a", "copy", "-c:v", "copy",
		destFile)

	var errText bytes.Buffer

	cmd.Stderr = &errText

	errLines := strings.Count(errText.String(), "\n")

	err := cmd.Run()
	if err != nil {
		return errLines, fmt.Errorf("run ffmpeg: %w, error text: %s", err,
			errText.String())
	}

	return errLines, nil
}

type Check struct {
	DurationSec      int `json:"duration_sec"`
	RTPMissedPackets int `json:"rtp_missed_packets"`
	CorruptedFrames  int `json:"corrupted_frames"`
	DecodingErrors   int `json:"decoding_errors"`
	MaxDelayReaches  int `json:"max_delay_reaches"`
}

func CheckStream(ffmpegPath, uri string, durationSec int) (c Check, err error) {
	c.DurationSec = durationSec

	cmd := exec.Command(ffmpegPath, "-v", "warning", "-i", uri,
		"-t", strconv.Itoa(durationSec), "-f", "null", "/dev/nulll")

	buf := bytes.NewBuffer(nil)

	cmd.Stderr = buf
	cmd.Stdout = buf

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("run ffmpeg: %w, output: %s", err,
			buf.String())
		return
	}

	s := bufio.NewScanner(buf)

	var (
		i          = -1
		lastParamI int
		lastParam  *int
	)

	for s.Scan() {
		i++
		l := s.Text()

		if strings.Contains(l, "RTP: missed ") {
			parts := strings.Split(l, " ")
			if len(parts) < 6 {
				continue
			}
			rtpMissedPackets, err := strconv.Atoi(parts[5])
			if err != nil {
				continue
			}

			c.RTPMissedPackets += rtpMissedPackets
			lastParam = &c.RTPMissedPackets
			lastParamI = i

			continue
		}

		if strings.Contains(l, "error while decoding") {
			c.DecodingErrors++
			lastParam = &c.DecodingErrors
			lastParamI = i
			continue
		}

		if strings.Contains(l, "max delay reached. need to consume packet") {
			c.MaxDelayReaches++
			lastParam = &c.DecodingErrors
			lastParamI = i
		}

		if strings.Contains(l, "corrupt decoded frame") {
			c.CorruptedFrames++
			lastParam = &c.CorruptedFrames
			lastParamI = i
		}

		if lastParamI < i-1 {
			lastParamI = 0
			lastParam = nil
		}

		if strings.Contains(l, "Last message repeated") && lastParam != nil {
			parts := strings.Split(strings.TrimSpace(l), " ")
			if len(parts) < 4 {
				continue
			}

			count, err := strconv.Atoi(parts[3])
			if err != nil {
				continue
			}
			*lastParam += count
		}
	}

	return
}
