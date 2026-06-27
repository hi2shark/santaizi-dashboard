package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/naiba/nezha/model"
	"github.com/naiba/nezha/pkg/mygin"
	"github.com/naiba/nezha/service/singleton"
)

type apiV1 struct {
	r gin.IRouter
}

func (v *apiV1) serve() {
	r := v.r.Group("")
	// 强制认证的 API
	r.Use(mygin.Authorize(mygin.AuthorizeOption{
		MemberOnly: true,
		AllowAPI:   true,
		IsPage:     false,
		Msg:        "访问此接口需要认证",
		Btn:        "点此登录",
		Redirect:   "/login",
	}))
	r.GET("/server/list", v.serverList)
	r.GET("/server/details", v.serverDetails)
	r.POST("/server/register", v.RegisterServer)
	r.GET("/offline-history", v.offlineHistory)
	r.GET("/offline-history/summary", v.offlineSummary)
	r.POST("/offline-history/cleanup", v.cleanupOfflineHistory)
	r.DELETE("/offline-history/:id", v.deleteOfflineHistory)
	// 不强制认证的 API
	mr := v.r.Group("monitor")
	mr.Use(mygin.Authorize(mygin.AuthorizeOption{
		MemberOnly: false,
		IsPage:     false,
		AllowAPI:   true,
		Msg:        "访问此接口需要认证",
		Btn:        "点此登录",
		Redirect:   "/login",
	}))
	mr.Use(mygin.ValidateViewPassword(mygin.ValidateViewPasswordOption{
		IsPage:        false,
		AbortWhenFail: true,
	}))
	mr.GET("/:id", v.monitorHistoriesById)

	sr := v.r.Group("server")
	sr.Use(mygin.Authorize(mygin.AuthorizeOption{
		MemberOnly: false,
		IsPage:     false,
		AllowAPI:   true,
		Msg:        "访问此接口需要认证",
		Btn:        "点此登录",
		Redirect:   "/login",
	}))
	sr.Use(mygin.ValidateViewPassword(mygin.ValidateViewPasswordOption{
		IsPage:        false,
		AbortWhenFail: true,
	}))
	sr.GET("/availability", v.serverAvailability)
}

// serverList 获取服务器列表 不传入Query参数则获取全部
// header: Authorization: Token
// query: tag (服务器分组)
func (v *apiV1) serverList(c *gin.Context) {
	tag := c.Query("tag")
	if tag != "" {
		c.JSON(200, singleton.ServerAPI.GetListByTag(tag))
		return
	}
	c.JSON(200, singleton.ServerAPI.GetAllList())
}

// serverDetails 获取服务器信息 不传入Query参数则获取全部
// header: Authorization: Token
// query: id (服务器ID，逗号分隔，优先级高于tag查询)
// query: tag (服务器分组)
func (v *apiV1) serverDetails(c *gin.Context) {
	var idList []uint64
	idListStr := strings.Split(c.Query("id"), ",")
	if c.Query("id") != "" {
		idList = make([]uint64, len(idListStr))
		for i, v := range idListStr {
			id, _ := strconv.ParseUint(v, 10, 64)
			idList[i] = id
		}
	}
	tag := c.Query("tag")
	if tag != "" {
		c.JSON(200, singleton.ServerAPI.GetStatusByTag(tag))
		return
	}
	if len(idList) != 0 {
		c.JSON(200, singleton.ServerAPI.GetStatusByIDList(idList))
		return
	}
	c.JSON(200, singleton.ServerAPI.GetAllStatus())
}

// RegisterServer adds a server and responds with the full ServerRegisterResponse
// header: Authorization: Token
// body: RegisterServer
// response: ServerRegisterResponse or Secret string
func (v *apiV1) RegisterServer(c *gin.Context) {
	var rs singleton.RegisterServer
	// Attempt to bind JSON to RegisterServer struct
	if err := c.ShouldBindJSON(&rs); err != nil {
		c.JSON(400, singleton.ServerRegisterResponse{
			CommonResponse: singleton.CommonResponse{
				Code:    400,
				Message: "Parse JSON failed",
			},
		})
		return
	}
	// Check if simple mode is requested
	simple := c.Query("simple") == "true" || c.Query("simple") == "1"
	// Set defaults if fields are empty
	if rs.Name == "" {
		rs.Name = c.ClientIP()
	}
	if rs.Tag == "" {
		rs.Tag = "AutoRegister"
	}
	if rs.HideForGuest == "" {
		rs.HideForGuest = "on"
	}
	// Call the Register function and get the response
	response := singleton.ServerAPI.Register(&rs)
	// Respond with Secret only if in simple mode, otherwise full response
	if simple {
		c.JSON(response.Code, response.Secret)
	} else {
		c.JSON(response.Code, response)
	}
}

func (v *apiV1) monitorHistoriesById(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(400, gin.H{"code": 400, "message": "id参数错误"})
		return
	}
	server, ok := singleton.ServerList[id]
	if !ok {
		c.AbortWithStatusJSON(404, gin.H{
			"code":    404,
			"message": "id不存在",
		})
		return
	}

	_, isMember := c.Get(model.CtxKeyAuthorizedUser)
	_, isViewPasswordVerfied := c.Get(model.CtxKeyViewPasswordVerified)
	authorized := isMember || isViewPasswordVerfied

	if server.HideForGuest && !authorized {
		c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "需要认证"})
		return
	}

	c.JSON(200, singleton.MonitorAPI.GetMonitorHistories(map[string]any{"server_id": server.ID}))
}

// serverAvailability 获取服务器可用性摘要（聚合统计，供前台展示）。
// query: id 服务器 ID，多个用逗号分隔；为空则返回所有可见服务器。
// query: days 统计天数，默认 30，最大 3660。
func (v *apiV1) serverAvailability(c *gin.Context) {
	if !singleton.Conf.ShowAvailabilityToGuest {
		c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "前台可用性展示已关闭"})
		return
	}

	_, isMember := c.Get(model.CtxKeyAuthorizedUser)
	_, isViewPasswordVerfied := c.Get(model.CtxKeyViewPasswordVerified)
	authorized := isMember || isViewPasswordVerfied

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}
	if days > maxOfflineSummaryDays {
		days = maxOfflineSummaryDays
	}

	idStr := c.Query("id")
	var requestedIDs []uint64
	if idStr != "" {
		for _, s := range strings.Split(idStr, ",") {
			id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
			if err == nil && id > 0 {
				requestedIDs = append(requestedIDs, id)
			}
		}
	}

	var serverList []*model.Server
	singleton.SortedServerLock.RLock()
	if authorized {
		serverList = singleton.SortedServerList
	} else {
		serverList = singleton.SortedServerListForGuest
	}
	singleton.SortedServerLock.RUnlock()

	visibleIDs := make([]uint64, 0, len(serverList))
	visibleSet := make(map[uint64]bool, len(serverList))
	for _, server := range serverList {
		visibleIDs = append(visibleIDs, server.ID)
		visibleSet[server.ID] = true
	}

	var queryIDs []uint64
	if len(requestedIDs) > 0 {
		queryIDs = make([]uint64, 0, len(requestedIDs))
		for _, id := range requestedIDs {
			if visibleSet[id] {
				queryIDs = append(queryIDs, id)
			}
		}
	} else {
		queryIDs = visibleIDs
	}

	if len(queryIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "result": []interface{}{}})
		return
	}

	summaries, _, err := singleton.GetServerAvailabilitySummaries(queryIDs, days)
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"code": 500, "message": "查询可用性数据失败"})
		return
	}

	items := make([]*singleton.ServerAvailability, 0, len(summaries))
	for _, id := range queryIDs {
		if summary, ok := summaries[id]; ok {
			items = append(items, summary)
		}
	}

	c.JSON(http.StatusOK, model.Response{
		Code:   http.StatusOK,
		Result: items,
	})
}


// offlineHistory 获取服务器离线历史列表
// header: Authorization: Token
// query: server_id (必填)
// query: page (默认 1)
// query: page_size (默认 20, 最大 100)
func (v *apiV1) offlineHistory(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Query("server_id"), 10, 64)
	if err != nil || serverID == 0 {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: "server_id 参数错误",
		})
		return
	}

	singleton.ServerLock.RLock()
	_, ok := singleton.ServerList[serverID]
	singleton.ServerLock.RUnlock()
	if !ok {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: "服务器不存在",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var histories []model.ServerOfflineHistory
	var total int64
	singleton.DB.Model(&model.ServerOfflineHistory{}).Where("server_id = ?", serverID).Count(&total)
	singleton.DB.Where("server_id = ?", serverID).Order("started_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&histories)

	items := make([]offlineHistoryItem, len(histories))
	for i, h := range histories {
		items[i] = offlineHistoryItem{
			ID:                h.ID,
			ServerID:          h.ServerID,
			StartedAt:         h.StartedAt,
			DetectedAt:        h.DetectedAt,
			EndedAt:           h.EndedAt,
			DurationSeconds:   h.DurationSeconds,
			Reason:            h.Reason,
			Status:            h.Status,
			ThresholdSeconds:  h.ThresholdSeconds,
			LastSeenAt:        h.LastSeenAt,
			LastBootTime:      h.LastBootTime,
			RecoveredBootTime: h.RecoveredBootTime,
			LastIP:            h.LastIP,
			RecoveredIP:       h.RecoveredIP,
		}
	}

	c.JSON(http.StatusOK, model.Response{
		Code:   http.StatusOK,
		Result: offlineHistoryResponse{Items: items, Total: total},
	})
}

// offlineSummary 获取服务器离线统计摘要
// header: Authorization: Token
// query: server_id (必填)
// query: days (默认 30, 最大 3660)
func (v *apiV1) offlineSummary(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Query("server_id"), 10, 64)
	if err != nil || serverID == 0 {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: "server_id 参数错误",
		})
		return
	}

	singleton.ServerLock.RLock()
	_, ok := singleton.ServerList[serverID]
	singleton.ServerLock.RUnlock()
	if !ok {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: "服务器不存在",
		})
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}
	if days > maxOfflineSummaryDays {
		days = maxOfflineSummaryDays
	}

	end := time.Now()
	start := end.AddDate(0, 0, -days)

	var histories []model.ServerOfflineHistory
	singleton.DB.Where("server_id = ? AND started_at >= ? AND started_at <= ?", serverID, start, end).Find(&histories)

	var totalSeconds uint64
	var longestSeconds uint64
	var rebootCount, networkCount, unknownCount int
	for _, h := range histories {
		duration := h.DurationSeconds
		if h.Status == model.OfflineHistoryStatusOpen {
			duration = uint64(end.Sub(h.StartedAt).Seconds())
		}
		totalSeconds += duration
		if duration > longestSeconds {
			longestSeconds = duration
		}
		switch h.Reason {
		case model.OfflineReasonMachineReboot:
			rebootCount++
		case model.OfflineReasonNetworkDisconnect:
			networkCount++
		case model.OfflineReasonUnknown:
			unknownCount++
		}
	}

	offlineCount := len(histories)
	// 可用率：服务器从未上报过数据（LastSeenAt 为空）时为空值（nil），
	// 与前台可用性口径一致；否则按离线时长折算（已上报且无离线为 100）。
	var availability *float64
	var rt model.ServerRuntime
	if err := singleton.DB.Select("last_seen_at").Where("server_id = ?", serverID).First(&rt).Error; err == nil && rt.LastSeenAt != nil {
		pct := 100.0
		if offlineCount > 0 {
			periodSeconds := uint64(end.Sub(start).Seconds())
			if totalSeconds < periodSeconds {
				pct = singleton.FormatAvailabilityPercent((1 - float64(totalSeconds)/float64(periodSeconds)) * 100)
			} else {
				pct = 0
			}
		}
		availability = &pct
	}

	c.JSON(http.StatusOK, model.Response{
		Code: http.StatusOK,
		Result: offlineSummaryResponse{
			ServerID:               serverID,
			Days:                   days,
			OfflineCount:           offlineCount,
			TotalOfflineSeconds:    totalSeconds,
			LongestOfflineSeconds:  longestSeconds,
			AvailabilityPercent:    availability,
			RebootCount:            rebootCount,
			NetworkDisconnectCount: networkCount,
			UnknownCount:           unknownCount,
		},
	})
}

// cleanupOfflineHistory 手动清理离线历史
// header: Authorization: Token
// body: { "before_days": 365 }
func (v *apiV1) cleanupOfflineHistory(c *gin.Context) {
	u := c.MustGet(model.CtxKeyAuthorizedUser).(*model.User)
	if !u.SuperAdmin {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusForbidden,
			Message: "无权操作",
		})
		return
	}

	var req cleanupOfflineHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("请求错误：%s", err),
		})
		return
	}
	if req.BeforeDays == 0 {
		req.BeforeDays = singleton.Conf.OfflineHistoryRetentionDays
	}
	if req.BeforeDays < 1 {
		req.BeforeDays = 1
	}

	before := time.Now().AddDate(0, 0, -int(req.BeforeDays))
	res := singleton.DB.Unscoped().Delete(&model.ServerOfflineHistory{}, "started_at < ?", before)
	if res.Error != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("数据库错误：%s", res.Error),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:   http.StatusOK,
		Result: map[string]int64{"deleted": res.RowsAffected},
	})
}

// deleteOfflineHistory 删除单条离线历史
// header: Authorization: Token
func (v *apiV1) deleteOfflineHistory(c *gin.Context) {
	u := c.MustGet(model.CtxKeyAuthorizedUser).(*model.User)
	if !u.SuperAdmin {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusForbidden,
			Message: "无权操作",
		})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: "错误的记录 ID",
		})
		return
	}

	if err := singleton.DB.Unscoped().Delete(&model.ServerOfflineHistory{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("数据库错误：%s", err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code: http.StatusOK,
	})
}
