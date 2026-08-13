package controller

import (
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	openapispec "github.com/hi2shark/santaizi-dashboard/openapi"
	"github.com/hi2shark/santaizi-dashboard/pkg/mygin"
	"github.com/hi2shark/santaizi-dashboard/resource"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"gopkg.in/yaml.v3"
)

func registerSPARoutes(r *gin.Engine) {
	r.GET("/openapi/v2.yaml", serveOpenAPIYAML)
	r.GET("/openapi/v2.json", serveOpenAPIJSON)

	for from, to := range map[string]string{
		"/server": "/admin/servers", "/server/offline-history": "/admin/servers", "/monitor": "/admin/services",
		"/notification": "/admin/notifications", "/ddns": "/admin/ddns", "/nat": "/admin/nat", "/telemetry": "/admin/telemetry",
		"/setting": "/admin/settings", "/login": "/admin/login", "/api": "/admin/api-tokens",
	} {
		target := to
		r.GET(from, func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, target) })
	}

	r.GET("/", serveWebApp("status", "/"))
	r.GET("/assets/*filepath", serveWebAsset("status"))
	r.GET("/service", serveWebApp("status", "/"))
	r.GET("/network", serveWebApp("status", "/"))
	r.GET("/network/:id", serveWebApp("status", "/"))
	r.GET("/server/:serverId", serveWebApp("status", "/"))
	r.GET("/view-password", serveWebApp("status", "/"))
	r.GET("/admin", func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, "/admin/") })
	r.GET("/admin/*path", serveWebApp("admin", "/admin/"))
	r.GET("/docs/api", func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, "/docs/api/") })
	r.GET("/docs/api/*path", serveWebApp("api-docs", "/docs/api/"))

	cp := &commonPage{r: r}
	publicWS := r.Group("")
	publicWS.Use(mygin.Authorize(mygin.AuthorizeOption{AllowAPI: true}))
	publicWS.Use(mygin.ValidateViewPassword(mygin.ValidateViewPasswordOption{AbortWhenFail: true}))
	publicWS.GET("/ws", cp.ws)
	publicWS.GET("/ws/v2/public/runtime", cp.ws)
	oauth := &oauth2controller{r: r}
	oauth.serve()
}

func serveOpenAPIYAML(c *gin.Context) {
	c.Header("Content-Type", "application/yaml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", openapispec.V2YAML)
}

func serveOpenAPIJSON(c *gin.Context) {
	var document map[string]any
	if err := yaml.Unmarshal(openapispec.V2YAML, &document); err != nil {
		writeV2Problem(c, 500, "openapi_conversion_failed", err.Error())
		return
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		writeV2Problem(c, 500, "openapi_conversion_failed", err.Error())
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/json; charset=utf-8", encoded)
}

func serveWebApp(app, prefix string) gin.HandlerFunc {
	return serveWeb(app, prefix, true)
}

func serveWebAsset(app string) gin.HandlerFunc {
	return serveWeb(app, "/", false)
}

func serveWeb(app, prefix string, fallbackIndex bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if singleton.Conf.Web.Delivery == "external" {
			c.Status(http.StatusNotFound)
			return
		}
		root, err := fs.Sub(resource.WebFS, "web/"+app)
		if err != nil {
			writeV2Problem(c, 500, "web_assets_unavailable", err.Error())
			return
		}
		requested := strings.TrimPrefix(c.Request.URL.Path, prefix)
		requested = strings.TrimPrefix(path.Clean("/"+requested), "/")
		if requested == "." || requested == "" {
			requested = "index.html"
		}
		info, statErr := fs.Stat(root, requested)
		if statErr != nil || info.IsDir() {
			if !fallbackIndex {
				writeV2Problem(c, http.StatusNotFound, "web_asset_not_found", "请求的前端资源不存在")
				return
			}
			requested = "index.html"
		}
		content, err := fs.ReadFile(root, requested)
		if err != nil {
			if !fallbackIndex {
				writeV2Problem(c, http.StatusNotFound, "web_asset_not_found", "请求的前端资源不存在")
				return
			}
			writeV2Problem(c, 503, "web_assets_not_built", "前端产物不存在，请先执行 pnpm build")
			return
		}
		if requested == "index.html" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if strings.Contains(path.Base(requested), "-") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=300")
		}
		contentType := mime.TypeByExtension(path.Ext(requested))
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		c.Data(http.StatusOK, contentType, content)
	}
}
