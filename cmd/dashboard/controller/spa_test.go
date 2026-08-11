package controller

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/model"
	openapispec "github.com/hi2shark/santaizi-dashboard/openapi"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"gopkg.in/yaml.v3"
)

func TestServeWebRegistersSPAAndV2Routes(t *testing.T) {
	original := singleton.Conf
	defer func() { singleton.Conf = original }()
	singleton.Conf = &model.Config{Site: model.SiteConfig{CookieName: "santaizi", Theme: "server-status", DashboardTheme: "spa"}, Web: model.WebConfig{Delivery: "embedded"}}
	server := ServeWeb(0)
	engine, ok := server.Handler.(*gin.Engine)
	if !ok {
		t.Fatal("HTTP handler is not a Gin engine")
	}
	want := map[string]bool{
		"GET /": false, "GET /admin/*path": false, "GET /docs/api/*path": false,
		"GET /openapi/v2.yaml": false, "GET /api/v2/auth/session": false,
		"GET /api/v2/admin/servers": false, "POST /api/v2/admin/telemetry/collectors": false,
		"GET /ws/v2/public/runtime": false,
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
	for _, route := range engine.Routes() {
		if strings.Contains(route.Path, "terminal") || strings.Contains(route.Path, "file-sessions") || strings.Contains(route.Path, "/tasks") {
			t.Errorf("removed remote capability route remains registered: %s %s", route.Method, route.Path)
		}
	}
}

func TestV2UnauthorizedUsesProblemDetails(t *testing.T) {
	original := singleton.Conf
	defer func() { singleton.Conf = original }()
	singleton.Conf = &model.Config{Site: model.SiteConfig{CookieName: "santaizi"}, Web: model.WebConfig{Delivery: "embedded"}}
	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/summary", nil)
	response := httptest.NewRecorder()
	ServeWeb(0).Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Fatalf("unexpected content type %q", contentType)
	}
	for _, field := range []string{`"code":"authentication_required"`, `"trace_id"`, `"status":401`} {
		if !strings.Contains(response.Body.String(), field) {
			t.Errorf("problem response is missing %s: %s", field, response.Body.String())
		}
	}
}

func TestSafeAppearanceValidation(t *testing.T) {
	for _, unsafe := range []string{
		`@import "https://example.com/theme.css";`,
		`body { background: url(//example.com/a.png) }`,
		`a { width: expression(alert(1)) }`,
		`</style><script>alert(1)</script>`,
	} {
		if !forbiddenCSS.MatchString(unsafe) {
			t.Errorf("unsafe CSS was accepted: %s", unsafe)
		}
	}
	if forbiddenCSS.MatchString(`:root { --brand: #2563eb } .status-panel { border-radius: 12px }`) {
		t.Fatal("safe CSS was rejected")
	}
	if got := safeAssetURL("https://example.com/logo.svg", "/static/logo.svg"); got != "/static/logo.svg" {
		t.Fatalf("external asset URL was accepted: %q", got)
	}
}

func TestOpenAPIV2Contract(t *testing.T) {
	var document struct {
		OpenAPI string                    `yaml:"openapi"`
		Paths   map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openapispec.V2YAML, &document); err != nil {
		t.Fatal(err)
	}
	if document.OpenAPI != "3.0.3" {
		t.Fatalf("unexpected OpenAPI version %q", document.OpenAPI)
	}
	for _, path := range []string{"/api/v2/auth/session", "/api/v2/public/servers", "/api/v2/admin/servers", "/api/v2/admin/telemetry/collectors"} {
		if _, ok := document.Paths[path]; !ok {
			t.Errorf("OpenAPI path %s is missing", path)
		}
	}
	original := singleton.Conf
	defer func() { singleton.Conf = original }()
	singleton.Conf = &model.Config{Site: model.SiteConfig{CookieName: "santaizi"}, Web: model.WebConfig{Delivery: "embedded"}}
	engine := ServeWeb(0).Handler.(*gin.Engine)
	registered := map[string]bool{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+normalizeContractPath(route.Path)] = true
	}
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	for routePath, operations := range document.Paths {
		for method := range operations {
			if !methods[strings.ToLower(method)] {
				continue
			}
			key := strings.ToUpper(method) + " " + normalizeContractPath(routePath)
			if !registered[key] {
				t.Errorf("OpenAPI operation %s is not registered", key)
			}
		}
	}
}

var contractParameter = regexp.MustCompile(`\{[^}]+\}|:[^/]+`)

func normalizeContractPath(value string) string {
	return contractParameter.ReplaceAllString(value, "{}")
}
