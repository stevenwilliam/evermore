package http

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/stevenwilliam/evermore/internal/platform/apierror"
)

// Limit describes one bucket.
type Limit struct {
	Name   string
	Burst  int
	Window time.Duration
}

// The endpoints that mint or spend credentials get the tightest buckets.
var (
	rlLogin    = Limit{"login", 10, time.Minute}
	rlRegister = Limit{"register", 5, time.Hour}
	rlRefresh  = Limit{"refresh", 60, time.Minute}
	rlWrite    = Limit{"write", 120, time.Minute}
)

// Limiter is a fixed-window counter keyed by client IP and bucket name.
//
// It belongs to a router instance rather than the package. A package-level
// global is shared by every router ever constructed, which makes the state
// untestable and would silently couple two servers in one process.
//
// In-process on purpose for now: this service runs as a single instance behind
// nginx, so a shared Redis bucket would add a dependency without changing the
// guarantee. When a second instance appears this must move to Redis, and the
// deployment handbook says so.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
}

// NewLimiter returns an empty limiter.
func NewLimiter() *Limiter { return &Limiter{windows: map[string]*window{}} }

type window struct {
	count int
	reset time.Time
}

func (l *Limiter) allow(key string, lim Limit, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[key]
	if !ok || now.After(w.reset) {
		w = &window{count: 0, reset: now.Add(lim.Window)}
		l.windows[key] = w
	}
	// Opportunistic sweep, so a long-running process does not accumulate a
	// bucket per IP that ever touched it.
	if len(l.windows) > 10000 {
		for k, v := range l.windows {
			if now.After(v.reset) {
				delete(l.windows, k)
			}
		}
	}
	w.count++
	if w.count > lim.Burst {
		return false, time.Until(w.reset)
	}
	return true, 0
}

// RateLimit enforces one bucket against a given limiter.
func RateLimit(l *Limiter, lim Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ClientIP is trustworthy here only because SetTrustedProxies limits
		// X-Forwarded-For to the loopback nginx.
		key := lim.Name + "|" + c.ClientIP()
		ok, retry := l.allow(key, lim, time.Now())
		if !ok {
			c.Header("Retry-After", itoa(int(retry.Seconds())+1))
			Fail(c, apierror.RateLimited("Terlalu banyak permintaan. Coba lagi sebentar."))
			return
		}
		c.Next()
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var _ = http.StatusTooManyRequests
