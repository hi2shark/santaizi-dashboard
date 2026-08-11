package mygin

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// RecordPath stores a parameter-normalized route for structured access logs.
func RecordPath(c *gin.Context) {
	value := c.Request.URL.Path
	for _, parameter := range c.Params {
		value = strings.Replace(value, parameter.Value, ":"+parameter.Key, 1)
	}
	c.Set("MatchedPath", value)
}
