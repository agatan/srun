package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agatan/srun"
	"github.com/agatan/srun/server/srun-server/middleware"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

func main() {
	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	runner := srun.New(client)

	r := gin.Default()
	r.Use(middleware.SetRunnerWrapper(runner))
	r.POST("/execute/sync", ExecuteSync)
	port := ":8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if err := http.ListenAndServe(port, r); err != nil {
		panic(err)
	}
}

type ExecuteRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

type ExecuteSyncResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitStatus int    `json:"exit_status"`
}

const executeSyncTimeout = 5 * time.Minute

func ExecuteSync(c *gin.Context) {
	var req ExecuteRequest

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	runner := middleware.GetRunner(c)
	lang, ok := runner.FindLanguageByName(req.Language)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%q is not supported", req.Language)})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), executeSyncTimeout)
	defer cancel()

	res, err := runner.Run(ctx, lang, req.Code)
	if err != nil {
		if errors.Cause(err) == context.DeadlineExceeded {
			c.JSON(http.StatusNotAcceptable, gin.H{"error": err})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err})
		}
		return
	}

	resp := ExecuteSyncResponse{
		Stdout:     string(res.Stdout),
		Stderr:     string(res.Stderr),
		ExitStatus: res.ExitStatus,
	}
	c.JSON(http.StatusOK, resp)
}
