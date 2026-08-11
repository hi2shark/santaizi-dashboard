package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"google.golang.org/grpc"
)

type controlSession struct {
	serverID     uint64
	stream       grpc.BidiStreamingServer[pb.AgentControlRequest, pb.PrimaryControlResponse]
	capabilities map[pb.AgentCapability]bool
	sendMu       sync.Mutex
}

func newControlSession(serverID uint64, hello *pb.AgentControlHello, stream grpc.BidiStreamingServer[pb.AgentControlRequest, pb.PrimaryControlResponse]) *controlSession {
	capabilities := make(map[pb.AgentCapability]bool)
	if hello.GetCapabilities() != nil {
		for _, capability := range hello.GetCapabilities().GetEnabled() {
			capabilities[capability] = true
		}
	}
	return &controlSession{serverID: serverID, stream: stream, capabilities: capabilities}
}

func (s *controlSession) send(response *pb.PrimaryControlResponse) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.stream.Send(response)
}

func (s *controlSession) supports(capability pb.AgentCapability) bool {
	return s.capabilities[capability]
}

var controlSessions = struct {
	sync.RWMutex
	items map[uint64]*controlSession
}{items: make(map[uint64]*controlSession)}

func registerControlSession(session *controlSession) {
	controlSessions.Lock()
	controlSessions.items[session.serverID] = session
	controlSessions.Unlock()
}

func unregisterControlSession(session *controlSession) {
	controlSessions.Lock()
	if controlSessions.items[session.serverID] == session {
		delete(controlSessions.items, session.serverID)
	}
	controlSessions.Unlock()
}

func controlSessionForServer(serverID uint64) *controlSession {
	controlSessions.RLock()
	defer controlSessions.RUnlock()
	return controlSessions.items[serverID]
}

type pendingProbe struct {
	monitorID uint64
	serverID  uint64
	expiresAt time.Time
}

var pendingProbes = struct {
	sync.Mutex
	items map[string]pendingProbe
}{items: make(map[string]pendingProbe)}

func monitorCapability(monitorType uint8) (pb.AgentCapability, bool) {
	switch monitorType {
	case model.MonitorTypeHTTPGet:
		return pb.AgentCapability_AGENT_CAPABILITY_PROBE_HTTP, true
	case model.MonitorTypeICMPPing:
		return pb.AgentCapability_AGENT_CAPABILITY_PROBE_ICMP, true
	case model.MonitorTypeTCPPing:
		return pb.AgentCapability_AGENT_CAPABILITY_PROBE_TCP, true
	default:
		return pb.AgentCapability_AGENT_CAPABILITY_UNSPECIFIED, false
	}
}

func monitorIncludesServer(monitor model.Monitor, serverID uint64) bool {
	inSet := monitor.SkipServers[serverID]
	if monitor.Cover == model.MonitorCoverIgnoreAll {
		return inSet
	}
	return !inSet
}

func probeRequestForMonitor(monitor model.Monitor, probeID string) (*pb.ProbeRequest, error) {
	target := strings.TrimSpace(monitor.Target)
	if target == "" {
		return nil, errors.New("monitor target is empty")
	}
	request := &pb.ProbeRequest{ProbeId: probeID}
	switch monitor.Type {
	case model.MonitorTypeHTTPGet:
		request.Target = &pb.ProbeRequest_Http{Http: &pb.HTTPProbeRequest{Url: target, TimeoutMs: 30000}}
	case model.MonitorTypeICMPPing:
		request.Target = &pb.ProbeRequest_Icmp{Icmp: &pb.ICMPProbeRequest{Host: target, Count: 5, TimeoutMs: 20000}}
	case model.MonitorTypeTCPPing:
		host, portText, err := net.SplitHostPort(target)
		if err != nil {
			return nil, fmt.Errorf("TCP monitor target must be host:port: %w", err)
		}
		var port uint64
		if _, err := fmt.Sscan(portText, &port); err != nil || port == 0 || port > 65535 {
			return nil, errors.New("TCP monitor port is invalid")
		}
		request.Target = &pb.ProbeRequest_Tcp{Tcp: &pb.TCPProbeRequest{Host: host, Port: uint32(port), TimeoutMs: 10000}}
	default:
		return nil, errors.New("unsupported monitor type")
	}
	return request, nil
}

func newProbeID(monitorID, serverID uint64) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%d-%s", monitorID, serverID, hex.EncodeToString(random)), nil
}

// DispatchMonitor sends a monitor only to explicitly selected, connected
// agents that advertised the required probe capability.
func DispatchMonitor(monitor model.Monitor) {
	capability, ok := monitorCapability(monitor.Type)
	if !ok {
		log.Printf("SANTAIZI>> unsupported monitor type %d", monitor.Type)
		return
	}
	controlSessions.RLock()
	sessions := make([]*controlSession, 0, len(controlSessions.items))
	for _, session := range controlSessions.items {
		sessions = append(sessions, session)
	}
	controlSessions.RUnlock()
	for _, session := range sessions {
		if !monitorIncludesServer(monitor, session.serverID) || !session.supports(capability) {
			continue
		}
		probeID, err := newProbeID(monitor.ID, session.serverID)
		if err != nil {
			log.Printf("SANTAIZI>> create probe ID: %v", err)
			continue
		}
		request, err := probeRequestForMonitor(monitor, probeID)
		if err != nil {
			log.Printf("SANTAIZI>> invalid monitor %d: %v", monitor.ID, err)
			continue
		}
		pendingProbes.Lock()
		now := time.Now()
		for id, pending := range pendingProbes.items {
			if now.After(pending.expiresAt) {
				delete(pendingProbes.items, id)
			}
		}
		pendingProbes.items[probeID] = pendingProbe{monitorID: monitor.ID, serverID: session.serverID, expiresAt: now.Add(2 * time.Minute)}
		pendingProbes.Unlock()
		if err := session.send(&pb.PrimaryControlResponse{Body: &pb.PrimaryControlResponse_ProbeRequest{ProbeRequest: request}}); err != nil {
			pendingProbes.Lock()
			delete(pendingProbes.items, probeID)
			pendingProbes.Unlock()
			log.Printf("SANTAIZI>> dispatch monitor %d to server %d: %v", monitor.ID, session.serverID, err)
		}
	}
}

func dispatchProbeResult(serverID uint64, result *pb.ProbeResult) {
	if result == nil || result.GetProbeId() == "" {
		return
	}
	pendingProbes.Lock()
	pending, ok := pendingProbes.items[result.GetProbeId()]
	if ok {
		delete(pendingProbes.items, result.GetProbeId())
	}
	pendingProbes.Unlock()
	if !ok || pending.serverID != serverID || time.Now().After(pending.expiresAt) || singleton.ServiceSentinelShared == nil {
		return
	}
	singleton.ServiceSentinelShared.Dispatch(singleton.ReportData{Data: result, MonitorID: pending.monitorID, Reporter: serverID})
}
