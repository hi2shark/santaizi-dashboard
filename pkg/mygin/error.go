package mygin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hi2shark/santaizi-dashboard/model"
)

type ErrInfo struct {
	Code  int
	Title string
	Msg   string
	Link  string
	Btn   string
}

func ShowErrorPage(c *gin.Context, i ErrInfo, isPage bool) {
	if isPage {
		if i.Link != "" {
			c.Redirect(http.StatusSeeOther, i.Link)
		} else {
			c.JSON(i.Code, gin.H{"title": i.Title, "status": i.Code, "code": "page_error", "detail": i.Msg})
		}
	} else {
		c.JSON(http.StatusOK, model.Response{
			Code:    i.Code,
			Message: i.Msg,
		})
	}
	c.Abort()
}
