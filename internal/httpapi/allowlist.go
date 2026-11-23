package httpapi

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Allowlist struct {
	networks []*net.IPNet
	domains  map[string]bool
}

func NewAllowlist(ips, domains []string) (*Allowlist, error) {
	a := &Allowlist{domains: map[string]bool{}}
	for _, v := range ips {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if ip := net.ParseIP(v); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
				ip = ip.To4()
			}
			a.networks = append(a.networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, e := net.ParseCIDR(v)
		if e != nil {
			return nil, fmt.Errorf("invalid allowed IP %q: %w", v, e)
		}
		a.networks = append(a.networks, n)
	}
	for _, v := range domains {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if strings.ContainsAny(v, "/:@ ") || strings.HasPrefix(v, ".") {
			return nil, fmt.Errorf("invalid allowed domain %q", v)
		}
		a.domains[v] = true
	}
	return a, nil
}
func (a *Allowlist) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip != nil {
			for _, n := range a.networks {
				if n.Contains(ip) {
					c.Next()
					return
				}
			}
		}
		// Origin is client-controlled and NOT authentication.
		if origin := c.GetHeader("Origin"); origin != "" {
			if u, err := url.Parse(origin); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && a.domains[strings.ToLower(u.Hostname())] {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}
