package controller

import (
	"fmt"
	"net/http"
	stdpprof "net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/pkg/mygin"
	"github.com/hi2shark/santaizi-dashboard/pkg/utils"
	"github.com/hi2shark/santaizi-dashboard/resource"
	"github.com/hi2shark/santaizi-dashboard/service/rpc"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
)

func ServeWeb(port uint) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	if singleton.Conf.Debug {
		gin.SetMode(gin.DebugMode)
		debug := r.Group("/debug")
		debug.Use(mygin.Authorize(mygin.AuthorizeOption{MemberOnly: true, Msg: "无权访问"}))
		pprofGroup := debug.Group("/pprof")
		pprofGroup.GET("/", gin.WrapF(stdpprof.Index))
		pprofGroup.GET("/cmdline", gin.WrapF(stdpprof.Cmdline))
		pprofGroup.GET("/profile", gin.WrapF(stdpprof.Profile))
		pprofGroup.GET("/symbol", gin.WrapF(stdpprof.Symbol))
		pprofGroup.POST("/symbol", gin.WrapF(stdpprof.Symbol))
		pprofGroup.GET("/trace", gin.WrapF(stdpprof.Trace))
		pprofGroup.GET("/allocs", gin.WrapH(stdpprof.Handler("allocs")))
		pprofGroup.GET("/block", gin.WrapH(stdpprof.Handler("block")))
		pprofGroup.GET("/goroutine", gin.WrapH(stdpprof.Handler("goroutine")))
		pprofGroup.GET("/heap", gin.WrapH(stdpprof.Handler("heap")))
		pprofGroup.GET("/mutex", gin.WrapH(stdpprof.Handler("mutex")))
		pprofGroup.GET("/threadcreate", gin.WrapH(stdpprof.Handler("threadcreate")))
	}
	r.StaticFS("/static", http.FS(resource.StaticFS))
	r.Use(mygin.CSRF(), mygin.RecordPath, mygin.Authorize(mygin.AuthorizeOption{}), natGateway)
	routes(r)
	missing := func(c *gin.Context) {
		writeV2Problem(c, http.StatusNotFound, "route_not_found", "请求的路由不存在")
	}
	r.NoRoute(missing)
	r.NoMethod(missing)
	return &http.Server{Addr: fmt.Sprintf(":%d", port), ReadHeaderTimeout: 5 * time.Second, Handler: r}
}

func routes(r *gin.Engine) {
	registerSPARoutes(r)
	api := r.Group("api")
	(&memberAPI{r: api}).serve()
}

// natGateway preserves the authenticated HTTP tunnel before SPA/API routing.
func natGateway(c *gin.Context) {
	natConfig := singleton.GetNATConfigByDomain(c.Request.Host)
	if natConfig == nil {
		return
	}
	if _, authorized := c.Get(model.CtxKeyAuthorizedUser); !authorized {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权访问"})
		return
	}
	if err := rpc.ValidateNATConnection(natConfig.ServerID, natConfig.Host); err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	w, err := utils.NewRequestWrapper(c.Request, c.Writer)
	if err != nil {
		_, _ = c.Writer.WriteString(fmt.Sprintf("request wrapper error: %v", err))
		c.Abort()
		return
	}
	_ = rpc.ConnectNAT(c.Request.Context(), natConfig.ServerID, natConfig.Host, w)
	c.Abort()
}
