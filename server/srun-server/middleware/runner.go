package middleware

import (
	"github.com/agatan/srun"
	"github.com/gin-gonic/gin"
)

const runnerContextName = "Runner"

func SetRunnerWrapper(r *srun.Runner) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(runnerContextName, r)
		c.Next()
	}
}

func GetRunner(c *gin.Context) *srun.Runner {
	return c.MustGet(runnerContextName).(*srun.Runner)
}
