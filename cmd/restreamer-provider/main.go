package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/lafikl/liblb"
	"github.com/lafikl/liblb/consistent"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	var (
		storageFile string
		bindAddr    string
	)

	kingpin.Flag("storage-file", "JSON storage file to use").
		Envar("STORAGE_FILE").StringVar(&storageFile)
	kingpin.Flag("bind-addr", "HTTP server bind address").
		Envar("BIND_ADDR").Default(":80").StringVar(&bindAddr)

	kingpin.Parse()

	var (
		mx    sync.RWMutex
		hosts []string
	)

	if _, err := os.Stat(storageFile); !os.IsNotExist(err) {
		hostsJSON, err := ioutil.ReadFile(storageFile)
		if err != nil {
			logrus.WithError(err).Fatal("failed to read storage file")
		}

		err = json.Unmarshal(hostsJSON, &hosts)
		if err != nil {
			logrus.WithError(err).Fatal("failed to unmarshal hosts file")
		}
	}

	balancer := consistent.New(hosts...)

	e := echo.New()

	e.Use(middleware.Recover())
	e.Use(logrusLogger)

	e.HideBanner = true
	e.HidePort = true

	e.HTTPErrorHandler = func(err error, ctx echo.Context) {
		var (
			code = http.StatusInternalServerError
			msg  interface{}
		)

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			msg = he.Message
		} else {
			msg = http.StatusText(code)
		}
		if _, ok := msg.(string); !ok {
			msg = fmt.Sprintf("%v", msg)
		}

		// Send response
		if !ctx.Response().Committed {
			if ctx.Request().Method == http.MethodHead { // Issue #608
				err = ctx.NoContent(code)
			} else {
				err = ctx.JSON(code, msg.(string))
			}
			if err != nil {
				logrus.WithError(err).Error("failed to error response")
			}
		}
	}

	e.GET("/host", func(c echo.Context) error {
		host, err := balancer.Balance(c.QueryParam("uri"))
		if err != nil {
			if errors.Is(err, liblb.ErrNoHost) {
				return echo.NewHTTPError(http.StatusNotFound,
					"no host available")
			}
			return err
		}

		return c.JSON(http.StatusOK, host)
	})

	e.GET("/hosts", func(c echo.Context) error {
		mx.RLock()
		defer mx.RUnlock()
		return c.JSON(http.StatusOK, hosts)
	})

	e.POST("/hosts", func(c echo.Context) error {
		host := c.QueryParam("host")
		if host == "" {
			return echo.NewHTTPError(http.StatusBadRequest,
				"host is empty")
		}

		mx.Lock()
		defer mx.Unlock()

		newHosts := append(hosts, host)

		err := writeStorageFile(newHosts, storageFile)
		if err != nil {
			return err
		}

		hosts = newHosts
		balancer.Add(host)

		return c.NoContent(http.StatusOK)
	})

	e.DELETE("/hosts", func(c echo.Context) error {
		host := c.QueryParam("host")

		mx.Lock()
		defer mx.Unlock()

		var newHosts []string

		for _, h := range hosts {
			if h == host {
				continue
			}
			newHosts = append(newHosts, h)
		}

		if len(newHosts) == len(hosts) {
			return echo.NewHTTPError(http.StatusNotFound,
				"host not found")
		}

		err := writeStorageFile(newHosts, storageFile)
		if err != nil {
			return err
		}

		hosts = newHosts
		balancer.Remove(host)

		return c.NoContent(http.StatusOK)
	})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := e.Start(bindAddr)
		if err != nil {
			if err == http.ErrServerClosed {
				return
			}
			logrus.WithError(err).Fatal("failed to start web server")
		}
	}()

	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	logrus.Infof("captured %v signal, stopping", <-signals)

	st := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := e.Shutdown(ctx)
	if err != nil {
		logrus.WithError(err).Error(
			"failed to graceful shutdown web server")
	}

	wg.Wait()

	logrus.Infof("stopped in %s seconds, exiting", time.Now().Sub(st))
}

func writeStorageFile(hosts []string, storageFile string) error {
	f, err := os.Create(storageFile + "-temp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	err = json.NewEncoder(f).Encode(hosts)
	if err != nil {
		return fmt.Errorf("JSON encode hosts to temp file: %w",
			err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	err = os.Rename(f.Name(), storageFile)
	if err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func logrusLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)

		stop := time.Now()

		if err != nil {
			c.Error(err)
		}

		req := c.Request()
		res := c.Response()

		p := req.URL.Path
		if p == "" {
			p = "/"
		}

		bytesIn := req.Header.Get(echo.HeaderContentLength)
		if bytesIn == "" {
			bytesIn = "0"
		}

		entry := logrus.WithFields(map[string]interface{}{
			"subsystem":  "web_server",
			"remote_ip":  c.RealIP(),
			"host":       req.Host,
			"uri":        req.RequestURI,
			"method":     req.Method,
			"path":       p,
			"referer":    req.Referer(),
			"user_agent": req.UserAgent(),
			"status":     res.Status,
			"latency":    stop.Sub(start).String(),
			"bytes_in":   bytesIn,
			"bytes_out":  strconv.FormatInt(res.Size, 10),
		})

		if len(c.QueryParams()) > 1 {
			entry = entry.WithField("query_params", c.QueryParams())
		}

		const msg = "request handled"

		if res.Status >= 500 {
			if err != nil {
				entry = entry.WithError(err)
			}
			entry.Error(msg)
		} else if res.Status >= 400 {
			if err != nil {
				entry = entry.WithError(err)
			}
			entry.Warn(msg)
		} else {
			entry.Info(msg)
		}

		return nil
	}
}
