package mygin

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
)

type AuthorizeOption struct {
	GuestOnly  bool
	MemberOnly bool
	IsPage     bool
	AllowAPI   bool
	Msg        string
	Redirect   string
	Btn        string
}

func Authorize(opt AuthorizeOption) func(*gin.Context) {
	return func(c *gin.Context) {
		var code = http.StatusForbidden
		if opt.GuestOnly {
			code = http.StatusBadRequest
		}

		commonErr := ErrInfo{
			Title: "访问受限",
			Code:  code,
			Msg:   opt.Msg,
			Link:  opt.Redirect,
			Btn:   opt.Btn,
		}
		var isLogin bool

		// 用户鉴权
		token, _ := c.Cookie(singleton.Conf.Site.CookieName)
		token = strings.TrimSpace(token)
		if token != "" {
			var u model.User
			if err := singleton.DB.Where("token = ?", token).First(&u).Error; err == nil {
				isLogin = u.TokenExpired.After(time.Now())
			}
			if isLogin {
				c.Set(model.CtxKeyAuthorizedUser, &u)
			}
		}

		// API鉴权
		if opt.AllowAPI {
			apiToken := c.GetHeader("Authorization")
			if strings.HasPrefix(strings.ToLower(apiToken), "bearer ") {
				apiToken = strings.TrimSpace(apiToken[len("bearer "):])
			}
			if apiToken != "" {
				var userID uint64
				var permission string
				active := false
				singleton.ApiLock.RLock()
				if record, ok := singleton.ApiTokenList[apiToken]; ok && record.IsActive() {
					active = true
					userID = record.UserID
					permission = record.NormalizedPermission()
				}
				singleton.ApiLock.RUnlock()
				if active {
					var u model.User
					if err := singleton.DB.Where("id = ?", userID).First(&u).Error; err == nil {
						isLogin = true
						c.Set(model.CtxKeyAuthorizedUser, &u)
						c.Set(model.CtxKeyIsAPI, true)
						c.Set(model.CtxKeyAPITokenPermission, permission)
					} else {
						isLogin = false
					}
				}
			}
		}

		// 已登录且只能游客访问
		if isLogin && opt.GuestOnly {
			ShowErrorPage(c, commonErr, opt.IsPage)
			return
		}

		// 未登录且需要登录
		if !isLogin && opt.MemberOnly {
			ShowErrorPage(c, commonErr, opt.IsPage)
			return
		}
	}
}

// RejectReadOnlyAPITokenWrites blocks mutating HTTP methods for read-only Bearer tokens.
// Cookie sessions are unaffected.
func RejectReadOnlyAPITokenWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAPI, _ := c.Get(model.CtxKeyIsAPI); isAPI != true {
			c.Next()
			return
		}
		perm, _ := c.Get(model.CtxKeyAPITokenPermission)
		if perm != model.ApiTokenPermissionRead {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		default:
			c.Header("Content-Type", "application/problem+json")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"type":   "https://santaizi.dev/problems/api_token_read_only",
				"title":  http.StatusText(http.StatusForbidden),
				"status": http.StatusForbidden,
				"code":   "api_token_read_only",
				"detail": "只读 API Token 不能执行写操作",
			})
		}
	}
}
