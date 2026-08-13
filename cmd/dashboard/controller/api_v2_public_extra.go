package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func publicServerIDVisible(c *gin.Context, id uint64) bool {
	for _, server := range publicServerSnapshot(c) {
		if server["id"] == id {
			return true
		}
	}
	return false
}

func publicNetworkHistoryItems(resp *singleton.MonitorInfoResponse) []any {
	if resp == nil || len(resp.Result) == 0 {
		return []any{}
	}
	converted := snakeValue(resp.Result)
	if items, ok := converted.([]any); ok && items != nil {
		return items
	}
	return []any{}
}

func publicRemainingBytes(used, quota uint64) uint64 {
	if quota <= used {
		return 0
	}
	return quota - used
}

func publicWarningBytes(quota uint64, warningPercent float64) uint64 {
	if quota == 0 || warningPercent <= 0 {
		return 0
	}
	return uint64(float64(quota) * warningPercent / 100)
}

func publicAvailabilityAllowed(c *gin.Context) bool {
	if singleton.Conf == nil || singleton.Conf.ShowAvailabilityToGuest {
		return true
	}
	if _, ok := c.Get(model.CtxKeyAuthorizedUser); ok {
		return true
	}
	return false
}

func v2PublicServerAvailability(c *gin.Context) {
	if !requireV2PublicAccess(c) {
		return
	}
	if !publicAvailabilityAllowed(c) {
		writeV2Problem(c, http.StatusForbidden, "availability_hidden", "前台可用性展示已关闭")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeV2Problem(c, http.StatusBadRequest, "invalid_server_id", "server_id 无效")
		return
	}
	if !publicServerIDVisible(c, id) {
		writeV2Problem(c, http.StatusNotFound, "server_not_found", "服务器不存在")
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}
	if days > maxOfflineSummaryDays {
		days = maxOfflineSummaryDays
	}
	summaries, _, err := singleton.GetServerAvailabilitySummaries([]uint64{id}, days)
	if err != nil {
		writeV2Problem(c, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	item := summaries[id]
	if item == nil {
		item = &singleton.ServerAvailability{ServerID: id, Days: days}
	}
	writeV2Data(c, http.StatusOK, item)
}

func clampPublicMetricWindow(resolution string, hours int) (string, int) {
	if resolution != "1h" {
		resolution = "1m"
	}
	if hours < 1 {
		hours = 24
	}
	maxDays := uint64(30)
	if singleton.Conf != nil {
		if resolution == "1h" {
			maxDays = singleton.Conf.Retention.StateOneHourDays
			if maxDays == 0 {
				maxDays = 365
			}
		} else {
			maxDays = singleton.Conf.Retention.StateOneMinuteDays
			if maxDays == 0 {
				maxDays = 30
			}
		}
	} else if resolution == "1h" {
		maxDays = 365
	}
	maxHours := int(maxDays) * 24
	if hours > maxHours {
		hours = maxHours
	}
	return resolution, hours
}

func decodePublicMetricPoints(rows []model.StateRollup) []gin.H {
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		var payload pb.StateRollupPayload
		if len(row.Payload) > 0 {
			if err := proto.Unmarshal(row.Payload, &payload); err != nil {
				continue
			}
		}
		avg := payload.GetAverage()
		if avg == nil {
			avg = &pb.State{}
		}
		items = append(items, gin.H{
			"window_start":  time.Unix(0, row.WindowStart).UTC().Format(time.RFC3339),
			"cpu":           avg.GetCpu(),
			"mem_used":      avg.GetMemUsed(),
			"disk_used":     avg.GetDiskUsed(),
			"net_in_speed":  avg.GetNetInSpeed(),
			"net_out_speed": avg.GetNetOutSpeed(),
			"net_in_total":  payload.GetNetInTotal(),
			"net_out_total": payload.GetNetOutTotal(),
		})
	}
	return items
}

func v2PublicMetrics(c *gin.Context) {
	if !requireV2PublicAccess(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeV2Problem(c, http.StatusBadRequest, "invalid_server_id", "server_id 无效")
		return
	}
	if !publicServerIDVisible(c, id) {
		writeV2Problem(c, http.StatusNotFound, "server_not_found", "服务器不存在")
		return
	}
	resolution, hours := clampPublicMetricWindow(c.DefaultQuery("resolution", "1m"), 0)
	if raw := c.Query("hours"); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed < 1 {
			writeV2Problem(c, http.StatusBadRequest, "invalid_hours", "hours 无效")
			return
		}
		resolution, hours = clampPublicMetricWindow(resolution, parsed)
	} else {
		resolution, hours = clampPublicMetricWindow(resolution, 24)
	}
	items := []gin.H{}
	var binding model.ServerNodeBinding
	if err := singleton.DB.Where("server_id = ? AND current = ?", id, true).Order("valid_from DESC").First(&binding).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			writeV2List(c, items, v2Meta{Page: 1, PageSize: 0, Total: 0})
			return
		}
		writeV2Problem(c, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	from := time.Now().Add(-time.Duration(hours) * time.Hour).UnixNano()
	var rows []model.StateRollup
	if err := singleton.DB.Where("node_uuid = ? AND resolution = ? AND window_start >= ?", binding.NodeUUID, resolution, from).
		Order("window_start ASC").Find(&rows).Error; err != nil {
		writeV2Problem(c, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	items = decodePublicMetricPoints(rows)
	writeV2List(c, items, v2Meta{Page: 1, PageSize: len(items), Total: int64(len(items))})
}
