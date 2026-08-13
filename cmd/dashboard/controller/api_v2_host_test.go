package controller

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/model"
)

func TestHostAdminDTOIncludesReportedAddresses(t *testing.T) {
	t.Parallel()
	if hostAdminDTO(nil) != nil {
		t.Fatal("nil host should stay nil")
	}

	empty := hostAdminDTO(&model.Host{Platform: "linux"})
	view, ok := empty.(map[string]any)
	if !ok {
		t.Fatalf("empty host type %T", empty)
	}
	if _, exists := view["ip"]; exists {
		t.Fatalf("empty IP should omit ip: %#v", view)
	}
	if view["Platform"] != "linux" {
		t.Fatalf("Platform=%v", view["Platform"])
	}

	dual := mustHostView(t, hostAdminDTO(&model.Host{
		Platform:    "linux",
		IP:          "192.0.2.10/2001:db8::10",
		CountryCode: "HK",
	}))
	if dual["ip"] != "192.0.2.10/2001:db8::10" || dual["ipv4"] != "192.0.2.10" || dual["ipv6"] != "2001:db8::10" {
		t.Fatalf("dual=%#v", dual)
	}

	v4 := mustHostView(t, hostAdminDTO(&model.Host{IP: "192.0.2.10"}))
	if v4["ipv4"] != "192.0.2.10" {
		t.Fatalf("v4=%#v", v4)
	}
	if _, exists := v4["ipv6"]; exists {
		t.Fatalf("v4-only should omit ipv6: %#v", v4)
	}

	v6 := mustHostView(t, hostAdminDTO(&model.Host{IP: "2001:db8::10"}))
	if v6["ipv6"] != "2001:db8::10" {
		t.Fatalf("v6=%#v", v6)
	}
	if _, exists := v6["ipv4"]; exists {
		t.Fatalf("v6-only should omit ipv4: %#v", v6)
	}
}

func TestHostJSONOmitsIPForPublic(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(&model.Host{Platform: "linux", IP: "192.0.2.10/2001:db8::10", CountryCode: "SG"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	if strings.Contains(encoded, "192.0.2.10") || strings.Contains(encoded, "2001:db8::10") {
		t.Fatalf("public host JSON leaked IP: %s", encoded)
	}
	if strings.Contains(encoded, `"ip"`) || strings.Contains(encoded, `"IP"`) {
		t.Fatalf("public host JSON should omit ip keys: %s", encoded)
	}
}

func TestPublicHostViewAPITokenIncludesIP(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	host := &model.Host{Platform: "linux", IP: "192.0.2.10/2001:db8::10"}

	anonymous, _ := gin.CreateTestContext(httptest.NewRecorder())
	if publicHostView(anonymous, host) != host {
		t.Fatal("anonymous public host must stay the raw Host pointer")
	}

	cookie, _ := gin.CreateTestContext(httptest.NewRecorder())
	cookie.Set(model.CtxKeyAuthorizedUser, &model.User{})
	if publicHostView(cookie, host) != host {
		t.Fatal("cookie session on public host must not expose IP fields")
	}

	token, _ := gin.CreateTestContext(httptest.NewRecorder())
	token.Set(model.CtxKeyAuthorizedUser, &model.User{})
	token.Set(model.CtxKeyIsAPI, true)
	view := mustHostView(t, publicHostView(token, host))
	if view["ip"] != "192.0.2.10/2001:db8::10" || view["ipv4"] != "192.0.2.10" || view["ipv6"] != "2001:db8::10" {
		t.Fatalf("token host=%#v", view)
	}
}

func mustHostView(t *testing.T, value any) map[string]any {
	t.Helper()
	view, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("host type %T", value)
	}
	return view
}
