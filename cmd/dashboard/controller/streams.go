package controller

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/pkg/utils"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"golang.org/x/sync/singleflight"
)

type commonPage struct {
	r            *gin.Engine
	requestGroup singleflight.Group
}

type Data struct {
	Now     int64           `json:"now,omitempty"`
	Servers []*model.Server `json:"servers,omitempty"`
}

var upgrader = websocket.Upgrader{ReadBufferSize: 32768, WriteBufferSize: 32768}

func (cp *commonPage) getServerStat(c *gin.Context, withPublicNote bool) ([]byte, error) {
	_, member := c.Get(model.CtxKeyAuthorizedUser)
	_, verified := c.Get(model.CtxKeyViewPasswordVerified)
	authorized := member || verified
	value, err, _ := cp.requestGroup.Do(fmt.Sprintf("serverStats::%t::%t", authorized, withPublicNote), func() (any, error) {
		singleton.SortedServerLock.RLock()
		defer singleton.SortedServerLock.RUnlock()
		source := singleton.SortedServerListForGuest
		if authorized {
			source = singleton.SortedServerList
		}
		servers := make([]*model.Server, 0, len(source))
		for _, running := range source {
			item := *running
			presentation := runtimeForServer(item)
			item.Secret, item.Note = "", ""
			if !withPublicNote {
				item.PublicNote = ""
			}
			item.Telemetry = &model.TelemetryPresentation{Host: presentation.HostState, Connectivity: presentation.Connectivity, Available: presentation.Availability, Coverage: presentation.Coverage}
			servers = append(servers, &item)
		}
		return utils.Json.Marshal(Data{Now: time.Now().UnixMilli(), Servers: servers})
	})
	if err != nil {
		return nil, err
	}
	return value.([]byte), nil
}

func (cp *commonPage) ws(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	count := 0
	for {
		stat, err := cp.getServerStat(c, false)
		if err != nil {
			<-ticker.C
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, stat); err != nil {
			return
		}
		count++
		if count%4 == 0 {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
		<-ticker.C
	}
}
