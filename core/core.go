package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/entity"
)

type DBStorage interface {
	Task(taskID string) (entity.Task, error)
	SetTask(t entity.Task) error
}

type TaskPublisher interface {
	PublishTask(entity.Task) error
}

type Core struct {
	dbs           DBStorage
	taskPublisher TaskPublisher
	echo          *echo.Echo
	log           *logrus.Entry
	stop          chan struct{}
	wg            sync.WaitGroup
}

func NewCore(dbs DBStorage, tp TaskPublisher, bindAddr, jwtSecret string) *Core {
	c := &Core{
		dbs:           dbs,
		taskPublisher: tp,
		log:           logrus.WithField("subsystem", "core"),
		stop:          make(chan struct{}),
	}

	e := echo.New()
	c.echo = e

	e.HideBanner = true
	e.HidePort = true

	e.HTTPErrorHandler = c.httpErrorHandler

	e.Use(middleware.Recover())
	e.Use(logrusLogger)
	e.Use(middleware.JWT([]byte(jwtSecret)))

	e.POST("/tasks", c.postTasks)
	e.POST("/tasks-batch", c.postTasksBatch)
	e.GET("/tasks/:task-id", c.getTask)
	e.GET("/tasks/:task-id/result", c.getTaskResult)

	c.wg.Add(1)
	go func() {
		c.wg.Done()

		for {
			select {
			case <-c.stop:
				return
			default:
			}

			err := e.Start(bindAddr)
			if err != nil {
				if err != http.ErrServerClosed {
					return
				}
				logrus.WithError(err).Error("failed to start web server")
				time.Sleep(time.Second)
			}
		}
	}()

	return c
}

func (cr *Core) Stop() (err error) {
	close(cr.stop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = cr.echo.Shutdown(ctx)
	if err != nil {
		err = fmt.Errorf("gracefull shutdown web server: %w", err)
	}

	cr.wg.Wait()

	return
}

func (cr *Core) publishTask(t entity.Task) (err error) {
	if t.Type == entity.ComplextTaskType {
		st := t.Payloads[len(t.Results)]
		st.ID = t.ID
		st.GeoLocation = t.GeoLocation
		st.Results = nil
		st.Result = nil
		defer func(subtaskIndex int) {
			if err == nil {
				logrus.WithFields(logrus.Fields{
					"task_id":       t.ID,
					"subtask_index": subtaskIndex,
				}).Info("complex task subtask published")
			}
		}(len(t.Results))
		t = st
	}
	return cr.taskPublisher.PublishTask(t)
}

func (cr *Core) HandleTaskResult(tr entity.TaskResult) (err error) {
	log := cr.log.WithField("task_id", tr.TaskID)

	log.Debug("task result received")

	t, err := cr.dbs.Task(tr.TaskID)
	if err != nil {
		logrus.WithError(err).Error("failed to get task from DB storage")
		if err == entity.ErrTaskNotFound {
			return nil
		}
		return err
	}

	tr.TaskID = ""

	if t.Type == entity.ComplextTaskType {
		t.Results = append(t.Results, tr)

		err = cr.dbs.SetTask(t)
		if err != nil {
			return err
		}

		if tr.Ok && len(t.Results) < len(t.Payloads) {
			err = cr.publishTask(t)
			if err != nil {
				return err
			}
		}

	} else {
		t.Result = &tr

		err = cr.dbs.SetTask(t)
		if err != nil {
			return err
		}
	}

	log.Debug("task result successfully handled")

	return
}

func (cr *Core) postTasks(c echo.Context) error {
	var t entity.Task

	err := json.NewDecoder(c.Request().Body).Decode(&t)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Errorf("JSON decode task: %w", err))
	}

	err = t.Validate()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Errorf("task validation: %w", err))
	}

	t.ID = uuid.New().String()
	t.Result = nil
	t.Results = nil

	err = cr.dbs.SetTask(t)
	if err != nil {
		return fmt.Errorf("set task in DB storage: %w", err)
	}

	err = cr.publishTask(t)
	if err != nil {
		return fmt.Errorf("publish task: %w", err)
	}

	return c.JSON(http.StatusOK, t.ID)
}

func (cr *Core) postTasksBatch(c echo.Context) error {
	var ts []entity.Task

	err := json.NewDecoder(c.Request().Body).Decode(&ts)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Errorf("JSON decode tasks: %w", err))
	}

	for i, t := range ts {
		err = t.Validate()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Errorf("task #%d validation: %w", i, err))
		}
	}

	var ids []string

	for _, t := range ts {
		t.ID = uuid.New().String()
		t.Result = nil
		t.Results = nil

		err = cr.dbs.SetTask(t)
		if err != nil {
			return fmt.Errorf("set task in DB storage: %w", err)
		}

		err = cr.publishTask(t)
		if err != nil {
			return fmt.Errorf("publish task: %w", err)
		}

		ids = append(ids, t.ID)
	}

	return c.JSON(http.StatusOK, ids)
}

func (cr *Core) getTask(c echo.Context) error {
	t, err := cr.dbs.Task(c.Param("task-id"))
	if err != nil {
		if errors.Is(err, entity.ErrTaskNotFound) {
			return echo.NewHTTPError(http.StatusNotFound,
				"task not found")
		}
		return fmt.Errorf("get task from DB storage: %w", err)
	}
	return c.JSON(http.StatusOK, t)
}

func (cr *Core) getTaskResult(c echo.Context) error {
	t, err := cr.dbs.Task(c.Param("task-id"))
	if err != nil {
		if errors.Is(err, entity.ErrTaskNotFound) {
			return echo.NewHTTPError(http.StatusNotFound,
				"task not found")
		}
		return fmt.Errorf("get task from DB storage: %w", err)
	}
	if t.Type == entity.ComplextTaskType {
		if len(t.Results) == 0 {
			return echo.NewHTTPError(http.StatusNotFound,
				"task result not found")
		}
		return c.JSON(http.StatusOK, t.Results)
	}
	if t.Result == nil {
		return echo.NewHTTPError(http.StatusNotFound,
			"task result not found")
	}
	return c.JSON(http.StatusOK, t.Result)
}

func (cr *Core) httpErrorHandler(err error, ctx echo.Context) {
	var (
		code = http.StatusInternalServerError
		msg  interface{}
	)

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		msg = he.Message
	} else if cr.echo.Debug {
		msg = err.Error()
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
			cr.log.WithError(err).Error("failed to error response")
		}
	}
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
