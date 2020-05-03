package ffmpeg

import (
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
