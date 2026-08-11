package rpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hi2shark/santaizi-dashboard/pkg/ddns"
	"github.com/hi2shark/santaizi-dashboard/pkg/geoip"
	"github.com/hi2shark/santaizi-dashboard/pkg/grpcx"
	"github.com/hi2shark/santaizi-dashboard/pkg/utils"

	"github.com/jinzhu/copier"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
)

var SantaiziHandlerSingleton *SantaiziHandler

type SantaiziHandler struct {
	pb.UnimplementedSantaiziServiceServer
	Auth          *authHandler
	ioStreams     map[string]*ioStreamContext
	ioStreamMutex *sync.RWMutex
}

func NewSantaiziHandler() *SantaiziHandler {
	return &SantaiziHandler{
		Auth:          &authHandler{},
		ioStreamMutex: new(sync.RWMutex),
		ioStreams:     make(map[string]*ioStreamContext),
	}
}

func (s *SantaiziHandler) ReportTask(c context.Context, r *pb.TaskResult) (*pb.Receipt, error) {
	var err error
	var clientID uint64
	if clientID, err = s.Auth.Check(c); err != nil {
		return nil, err
	}
	if r.GetType() == model.TaskTypeCommand {
		// 处理上报的计划任务
		singleton.CronLock.RLock()
		defer singleton.CronLock.RUnlock()
		cr := singleton.Crons[r.GetId()]
		if cr != nil {
			singleton.ServerLock.RLock()
			defer singleton.ServerLock.RUnlock()
			// 保存当前服务器状态信息
			curServer := model.Server{}
			_ = copier.Copy(&curServer, singleton.ServerList[clientID])
			if cr.PushSuccessful && r.GetSuccessful() {
				singleton.SendNotification(cr.NotificationTag, fmt.Sprintf("[%s] %s, %s\n%s", singleton.Localizer.MustLocalize(
					&i18n.LocalizeConfig{
						MessageID: "ScheduledTaskExecutedSuccessfully",
					},
				), cr.Name, singleton.ServerList[clientID].Name, r.GetData()), nil, &curServer)
			}
			if !r.GetSuccessful() {
				singleton.SendNotification(cr.NotificationTag, fmt.Sprintf("[%s] %s, %s\n%s", singleton.Localizer.MustLocalize(
					&i18n.LocalizeConfig{
						MessageID: "ScheduledTaskExecutedFailed",
					},
				), cr.Name, singleton.ServerList[clientID].Name, r.GetData()), nil, &curServer)
			}
			singleton.DB.Model(cr).Updates(model.Cron{
				LastExecutedAt: time.Now().Add(time.Second * -1 * time.Duration(r.GetDelay())), // #nosec G115 -- delay is seconds, safely within range
				LastResult:     r.GetSuccessful(),
			})
		}
	} else if model.IsServiceSentinelNeeded(r.GetType()) {
		singleton.ServiceSentinelShared.Dispatch(singleton.ReportData{
			Data:     r,
			Reporter: clientID,
		})
	}
	return &pb.Receipt{Proced: true}, nil
}

func (s *SantaiziHandler) RequestTask(h *pb.Host, stream pb.SantaiziService_RequestTaskServer) error {
	var clientID uint64
	var err error
	if clientID, err = s.Auth.Check(stream.Context()); err != nil {
		return err
	}
	closeCh := make(chan error)
	singleton.ServerLock.RLock()
	singleton.ServerList[clientID].TaskCloseLock.Lock()
	// 修复不断的请求 task 但是没有 return 导致内存泄漏
	if singleton.ServerList[clientID].TaskClose != nil {
		close(singleton.ServerList[clientID].TaskClose)
	}
	singleton.ServerList[clientID].TaskStream = stream
	singleton.ServerList[clientID].TaskClose = closeCh
	singleton.ServerList[clientID].TaskCloseLock.Unlock()
	singleton.ServerLock.RUnlock()
	return <-closeCh
}

func (s *SantaiziHandler) ReportSystemState(c context.Context, r *pb.State) (*pb.Receipt, error) {
	var clientID uint64
	var err error
	if clientID, err = s.Auth.Check(c); err != nil {
		return nil, err
	}
	state := model.PB2State(r)

	// 在闭包里持锁，确保 panic 时也能释放锁；DB/通知等耗时操作放在锁外
	if err := func() error {
		singleton.ServerLock.Lock()
		defer singleton.ServerLock.Unlock()
		server := singleton.ServerList[clientID]
		if server == nil {
			return status.Errorf(codes.NotFound, "server not found")
		}
		server.LastActive = time.Now()
		server.State = &state

		// 应对 dashboard 重启的情况，如果从未记录过，先打点，等到小时时间点时入库
		if server.PrevTransferInSnapshot == 0 || server.PrevTransferOutSnapshot == 0 {
			server.PrevTransferInSnapshot = int64(state.NetInTransfer)   // #nosec G115 -- network transfer fits in int64
			server.PrevTransferOutSnapshot = int64(state.NetOutTransfer) // #nosec G115 -- network transfer fits in int64
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	// 更新持久化运行态与离线历史，必须在释放 ServerLock 后再执行，避免锁内执行耗时操作阻塞上报
	singleton.UpdateServerRuntimeOnStateReport(clientID, state)

	return &pb.Receipt{Proced: true}, nil
}

func (s *SantaiziHandler) ReportSystemInfo(c context.Context, r *pb.Host) (*pb.Receipt, error) {
	var clientID uint64
	var err error
	if clientID, err = s.Auth.Check(c); err != nil {
		return nil, err
	}
	host := model.PB2Host(r)

	// 在闭包里持锁，确保 panic 时也能释放锁；锁外执行 DDNS、通知和 DB 操作
	var (
		enableDDNS      bool
		ddnsProfiles    []uint64
		oldHostIP       string
		serverName      string
		ignoredIPNotify bool
	)
	if err := func() error {
		singleton.ServerLock.Lock()
		defer singleton.ServerLock.Unlock()
		server := singleton.ServerList[clientID]
		if server == nil {
			return status.Errorf(codes.NotFound, "server not found")
		}
		enableDDNS = server.EnableDDNS
		if len(server.DDNSProfiles) > 0 {
			ddnsProfiles = make([]uint64, len(server.DDNSProfiles))
			copy(ddnsProfiles, server.DDNSProfiles)
		}
		serverName = server.Name
		ignoredIPNotify = singleton.Conf.IgnoredIPNotificationServerIDs[clientID]
		if server.Host != nil {
			oldHostIP = server.Host.IP
		}

		/**
		 * 这里的 singleton 中的数据都是关机前的旧数据
		 * 当 agent 重启时，bootTime 变大，agent 端会先上报 host 信息，然后上报 state 信息
		 * 这是可以借助上报顺序的空档，将停机前的流量统计数据标记下来，加到下一个小时的数据点上
		 */
		if server.Host != nil && server.State != nil && server.Host.BootTime < host.BootTime {
			server.PrevTransferInSnapshot = server.PrevTransferInSnapshot - int64(server.State.NetInTransfer)    // #nosec G115 -- network transfer fits in int64
			server.PrevTransferOutSnapshot = server.PrevTransferOutSnapshot - int64(server.State.NetOutTransfer) // #nosec G115 -- network transfer fits in int64
		}

		// 不要冲掉国家码
		if server.Host != nil {
			host.CountryCode = server.Host.CountryCode
		}
		server.Host = &host
		return nil
	}(); err != nil {
		return nil, err
	}

	// 检查并更新 DDNS（在锁外执行）
	if enableDDNS && host.IP != "" && oldHostIP != host.IP {
		ipv4, ipv6, _ := utils.SplitIPAddr(host.IP)
		providers, err := singleton.GetDDNSProvidersFromProfiles(ddnsProfiles, &ddns.IP{Ipv4Addr: ipv4, Ipv6Addr: ipv6})
		if err == nil {
			for _, provider := range providers {
				go func(provider *ddns.Provider) {
					provider.UpdateDomain(c)
				}(provider)
			}
		} else {
			log.Printf("SANTAIZI>> 获取DDNS配置时发生错误: %v", err)
		}
	}

	// 发送 IP 变动通知（在锁外执行）
	if oldHostIP != "" && host.IP != "" && oldHostIP != host.IP && singleton.Conf.EnableIPChangeNotification &&
		((singleton.Conf.Cover == model.ConfigCoverAll && !ignoredIPNotify) ||
			(singleton.Conf.Cover == model.ConfigCoverIgnoreAll && ignoredIPNotify)) {
		singleton.SendNotification(singleton.Conf.IPChangeNotificationTag,
			fmt.Sprintf(
				"[%s] %s, %s => %s",
				singleton.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "IPChanged",
				}),
				serverName, singleton.IPDesensitize(oldHostIP),
				singleton.IPDesensitize(host.IP),
			),
			nil)
	}

	// 更新持久化运行态与离线历史，必须在释放 ServerLock 后再执行
	singleton.UpdateServerRuntimeOnHostReport(clientID, host)

	return &pb.Receipt{Proced: true}, nil
}

func (s *SantaiziHandler) IOStream(stream pb.SantaiziService_IOStreamServer) error {
	if _, err := s.Auth.Check(stream.Context()); err != nil {
		return err
	}
	id, err := stream.Recv()
	if err != nil {
		return err
	}
	if id == nil || len(id.Data) < 4 || (id.Data[0] != 0xff && id.Data[1] != 0x05 && id.Data[2] != 0xff && id.Data[3] == 0x05) {
		return fmt.Errorf("invalid stream id")
	}

	streamId := string(id.Data[4:])

	if _, err := s.GetStream(streamId); err != nil {
		return err
	}
	iw := grpcx.NewIOStreamWrapper(stream)
	if err := s.AgentConnected(streamId, iw); err != nil {
		return err
	}
	iw.Wait()
	return nil
}

func (s *SantaiziHandler) LookupGeoIP(c context.Context, r *pb.GeoIP) (*pb.GeoIP, error) {
	var clientID uint64
	var err error
	if clientID, err = s.Auth.Check(c); err != nil {
		return nil, err
	}

	// 根据内置数据库查询 IP 地理位置
	record := &geoip.IPInfo{}
	ip := r.GetIp()
	netIP := net.ParseIP(ip)
	location, err := geoip.Lookup(netIP, record)
	if err != nil {
		return nil, err
	}

	// 将地区码写入到 Host（写操作需使用 Lock）
	singleton.ServerLock.Lock()
	defer singleton.ServerLock.Unlock()
	if singleton.ServerList[clientID].Host == nil {
		return nil, status.Errorf(codes.NotFound, "host not found")
	}
	singleton.ServerList[clientID].Host.CountryCode = location

	return &pb.GeoIP{Ip: ip, CountryCode: location}, nil
}
