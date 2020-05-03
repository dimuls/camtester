package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/camtester/core/entity"
)

type DBStorage interface {
	AddTask(t entity.Task) (id int, err error)
	TaskResult(taskID int) (tr entity.TaskResult, err error)
	AddTaskResult(tr entity.TaskResult) (err error)
}

type TaskPublisher interface {
	PublishTask(entity.Task) error
}

type Core struct {
	dbStorage     DBStorage
	taskPublisher TaskPublisher
	echo          *echo.Echo
	log           *logrus.Entry
	stop          chan struct{}
	wg            sync.WaitGroup
}

func NewCore(dbs DBStorage, tp TaskPublisher, bindAddr, jwtSecret string) *Core {
	c := &Core{
		dbStorage:     dbs,
		taskPublisher: tp,
		log:           logrus.WithField("subsystem", "core"),
		stop:          make(chan struct{}),
	}

	e := echo.New()
	c.echo = e

	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(logrusLogger)
	e.Use(middleware.JWT([]byte(jwtSecret)))

	e.POST("/task", c.postTask)
	e.GET("/task/:task-id/result", c.getTaskResult)

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

func (cr *Core) HandleTaskResult(tr entity.TaskResult) (err error) {
	err = cr.dbStorage.AddTaskResult(tr)
	if err != nil {
		cr.log.WithError(err).Error("failed to add task result to DB storage")
	}
	return
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

func (cr *Core) postTask(c echo.Context) error {
	var t entity.Task

	err := c.Bind(&t)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Errorf("bind task: %w", err))
	}

	id, err := cr.dbStorage.AddTask(t)
	if err != nil {
		return fmt.Errorf("add task to DB storage: %w", err)
	}

	t.ID = id

	err = cr.taskPublisher.PublishTask(t)
	if err != nil {
		return fmt.Errorf("publish task: %w", err)
	}

	return c.JSON(http.StatusOK, id)
}

func (cr *Core) getTaskResult(c echo.Context) error {
	taskID, err := strconv.Atoi(c.Param("task-id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Errorf("parse task-id: %w", err))
	}

	tr, err := cr.dbStorage.TaskResult(taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound,
				"task result not found")
		}
		return fmt.Errorf("get task result from DB storage: %w", err)
	}

	return c.JSON(http.StatusOK, tr)
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
