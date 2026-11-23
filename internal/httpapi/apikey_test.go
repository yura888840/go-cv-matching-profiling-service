package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := "0123456789abcdef0123456789abcdef0123456789"
	r := gin.New()
	r.GET("/protected", APIKeyMiddleware(key), func(c *gin.Context) { c.Status(200) })
	for _, tc := range []struct {
		key  string
		want int
	}{
		{key, 200}, {"", 401}, {"wrong", 401},
	} {
		req := httptest.NewRequest("GET", "/protected", nil)
		if tc.key != "" {
			req.Header.Set("X-API-Key", tc.key)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("got %d want %d", w.Code, tc.want)
		}
	}
}
func TestAPIKeyMissingConfig(t *testing.T) {
	r := gin.New()
	r.GET("/", APIKeyMiddleware(""), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "anything")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}
