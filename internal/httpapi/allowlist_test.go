package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a, e := NewAllowlist([]string{"192.0.2.10", "10.0.0.0/8"}, []string{"app.example.com"})
	if e != nil {
		t.Fatal(e)
	}
	r := gin.New()
	r.Use(a.Middleware())
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	for _, tc := range []struct {
		addr, origin string
		status       int
	}{{"192.0.2.10:42", "", 200}, {"10.1.2.3:42", "", 200}, {"203.0.113.2:42", "", 403}, {"203.0.113.2:42", "https://app.example.com", 200}, {"203.0.113.2:42", "https://evil.example.com", 403}} {
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = tc.addr
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Errorf("%+v got %d", tc, w.Code)
		}
	}
}
