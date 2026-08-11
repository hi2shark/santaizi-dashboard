package mygin

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecordPathNormalizesParameters(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/v2/admin/servers/42", nil)
	context.Params = gin.Params{{Key: "id", Value: "42"}}
	RecordPath(context)
	value, _ := context.Get("MatchedPath")
	if value != "/api/v2/admin/servers/:id" {
		t.Fatalf("unexpected matched path %q", value)
	}
}
