package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"google.golang.org/protobuf/proto"
)

func TestPublicNetworkHistoryItemsAreArray(t *testing.T) {
	t.Parallel()
	if got := publicNetworkHistoryItems(nil); len(got) != 0 {
		t.Fatalf("nil resp=%#v", got)
	}
	resp := &singleton.MonitorInfoResponse{
		CommonResponse: singleton.CommonResponse{Code: 0, Message: "success"},
		Result: []*singleton.MonitorInfo{{
			MonitorID: 9, ServerID: 2, MonitorName: "ICMP", ServerName: "edge",
			CreatedAt: []int64{1_700_000_000_000}, AvgDelay: []float32{12.5},
		}},
	}
	items := publicNetworkHistoryItems(resp)
	if len(items) != 1 {
		t.Fatalf("len=%d items=%#v", len(items), items)
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("type %T", items[0])
	}
	if row["monitor_name"] != "ICMP" {
		t.Fatalf("monitor_name=%#v", row)
	}
	if _, exists := row["code"]; exists {
		t.Fatalf("legacy envelope leaked into items: %#v", row)
	}
	encoded, err := json.Marshal(map[string]any{"data": items})
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0]["monitor_name"] != "ICMP" {
		t.Fatalf("json=%s", encoded)
	}
}

func TestPublicCycleTransferDerivedFields(t *testing.T) {
	t.Parallel()
	if publicRemainingBytes(80, 100) != 20 || publicRemainingBytes(120, 100) != 0 {
		t.Fatal("remaining bytes")
	}
	if publicWarningBytes(1000, 80) != 800 {
		t.Fatalf("warning=%d", publicWarningBytes(1000, 80))
	}
}

func TestPublicAvailabilityAllowed(t *testing.T) {
	original := singleton.Conf
	t.Cleanup(func() { singleton.Conf = original })
	gin.SetMode(gin.TestMode)

	singleton.Conf = &model.Config{ShowAvailabilityToGuest: true}
	guest, _ := gin.CreateTestContext(httptest.NewRecorder())
	if !publicAvailabilityAllowed(guest) {
		t.Fatal("guest should read when switch is on")
	}

	singleton.Conf = &model.Config{ShowAvailabilityToGuest: false}
	if publicAvailabilityAllowed(guest) {
		t.Fatal("anonymous guest must be forbidden when switch is off")
	}
	admin, _ := gin.CreateTestContext(httptest.NewRecorder())
	admin.Set(model.CtxKeyAuthorizedUser, &model.User{Login: "admin"})
	if !publicAvailabilityAllowed(admin) {
		t.Fatal("admin must still read availability")
	}
	verified, _ := gin.CreateTestContext(httptest.NewRecorder())
	verified.Set(model.CtxKeyViewPasswordVerified, true)
	if publicAvailabilityAllowed(verified) {
		t.Fatal("view-password guest must follow the switch")
	}
}

func TestClampPublicMetricWindow(t *testing.T) {
	original := singleton.Conf
	t.Cleanup(func() { singleton.Conf = original })
	singleton.Conf = &model.Config{Retention: model.RetentionConfig{StateOneMinuteDays: 2, StateOneHourDays: 3}}

	resolution, hours := clampPublicMetricWindow("1m", 24)
	if resolution != "1m" || hours != 24 {
		t.Fatalf("default 1m: %s %d", resolution, hours)
	}
	resolution, hours = clampPublicMetricWindow("weird", 0)
	if resolution != "1m" || hours != 24 {
		t.Fatalf("invalid: %s %d", resolution, hours)
	}
	resolution, hours = clampPublicMetricWindow("1m", 10_000)
	if resolution != "1m" || hours != 48 {
		t.Fatalf("1m clamp: %s %d", resolution, hours)
	}
	resolution, hours = clampPublicMetricWindow("1h", 10_000)
	if resolution != "1h" || hours != 72 {
		t.Fatalf("1h clamp: %s %d", resolution, hours)
	}
}

func TestDecodePublicMetricPoints(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	payload, err := proto.Marshal(&pb.StateRollupPayload{
		Average:    &pb.State{Cpu: 12.5, MemUsed: 1024, DiskUsed: 2048, NetInSpeed: 10, NetOutSpeed: 20, ProcessCount: 88, TcpConnCount: 12, UdpConnCount: 4},
		NetInTotal: 100, NetOutTotal: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	items := decodePublicMetricPoints([]model.StateRollup{{
		Resolution:  "1m",
		WindowStart: start.UnixNano(),
		Payload:     payload,
	}})
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0]["cpu"] != 12.5 || items[0]["mem_used"] != uint64(1024) || items[0]["net_in_total"] != uint64(100) {
		t.Fatalf("item=%#v", items[0])
	}
	if items[0]["window_start"] != start.Format(time.RFC3339) {
		t.Fatalf("window_start=%v", items[0]["window_start"])
	}
	if items[0]["net_in_speed"] != uint64(10) || items[0]["net_out_speed"] != uint64(20) {
		t.Fatalf("speed=%#v", items[0])
	}
	if items[0]["process_count"] != uint64(88) || items[0]["tcp_conn_count"] != uint64(12) || items[0]["udp_conn_count"] != uint64(4) {
		t.Fatalf("counts=%#v", items[0])
	}
}

func TestDecodePublicMetricPointsDerivesSpeedFromWindowTotals(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	payload, err := proto.Marshal(&pb.StateRollupPayload{
		Average:    &pb.State{Cpu: 4, NetInSpeed: 0, NetOutSpeed: 0},
		NetInTotal: 6000, NetOutTotal: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	items := decodePublicMetricPoints([]model.StateRollup{{
		Resolution:  "1m",
		WindowStart: start.UnixNano(),
		WindowEnd:   start.Add(time.Minute).UnixNano(),
		Payload:     payload,
	}})
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0]["net_in_speed"] != uint64(100) || items[0]["net_out_speed"] != uint64(50) {
		t.Fatalf("derived speed=%#v", items[0])
	}
}

func TestPublicMetricAndAvailabilityRoutesRegistered(t *testing.T) {
	handler := withEmbeddedWeb(t)
	engine, ok := handler.(*gin.Engine)
	if !ok {
		t.Fatal("handler is not gin engine")
	}
	want := map[string]bool{
		"GET /api/v2/public/metrics/:id":              false,
		"GET /api/v2/public/servers/:id/availability": false,
		"GET /api/v2/public/network/:id":              false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := want[key]; exists {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("route %s is not registered", route)
		}
	}
}

func TestPublicNetworkHistoryJSONShape(t *testing.T) {
	items := publicNetworkHistoryItems(&singleton.MonitorInfoResponse{
		Result: []*singleton.MonitorInfo{{MonitorName: "TCP", CreatedAt: []int64{1}, AvgDelay: []float32{3}}},
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writeV2List(ctx, items, v2Meta{Page: 1, PageSize: len(items), Total: int64(len(items))})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0]["monitor_name"] != "TCP" {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}
