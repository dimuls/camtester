package main

import (
	"bufio"
	"flag"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gonum/stat"
)

func main() {
	var (
		lag                  int
		threshold, influence float64
	)

	flag.IntVar(&lag, "l", 0, "lag")
	flag.Float64Var(&threshold, "t", 0, "theshold")
	flag.Float64Var(&influence, "i", 0, "influence")

	flag.Parse()

	var samples []float64

	r := bufio.NewReader(os.Stdin)

	for {
		l, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logrus.WithError(err).Fatal("failed to read line")
		}

		l = strings.TrimSpace(l)

		s, err := strconv.ParseFloat(l, 64)
		if err != nil {
			logrus.WithError(err).Fatal("failed to parse sample")
		}

		samples = append(samples, s)
	}

	zScore := zScore(samples, lag, threshold, influence)

	var (
		top       int
		isPrevTop bool
	)

	for i, s := range zScore {
		if s == 1 || s == -1 {
			from := i - lag
			if from < 0 {
				from = 0
			}
			to := i + lag
			if to > len(zScore) {
				to = len(zScore)
			}

			logrus.WithFields(logrus.Fields{
				"index":       i,
				"is_prev_top": isPrevTop,
			}).Info("found top")

			if !isPrevTop {
				top++
				isPrevTop = true

				logrus.WithFields(logrus.Fields{
					"context": samples[from:to],
					"index":   i,
				}).Info("got top")
			}
		} else {
			isPrevTop = false
		}
	}

	logrus.WithField("top", top).Info("got TOP")
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
