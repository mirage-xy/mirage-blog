package web

import (
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

// chain 按传入顺序由外到内包住 handler，顺序固定见《开发规范》§4.1.5。
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// recoverMW 兜住 handler 里的 panic：记一条带堆栈的 error 日志，返回 500。
func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.logger.Error("panic",
					"value", v,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logMW 记一条访问日志。日志放在 defer 里，handler panic 时这条也能留下。
func (s *Server) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			s.logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"dur_ms", time.Since(start).Milliseconds(),
				"ip", clientIP(r),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}

// statusRecorder 记状态码和响应字节数，只给访问日志用。
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap 让 http.ResponseController 能拿到底层 ResponseWriter。
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// clientIP 取真实 IP：服务在 Nginx 后面，RemoteAddr 恒为 127.0.0.1。
// 只认 Nginx 覆盖写入的 X-Real-IP，不认 X-Forwarded-For——后者客户端能自己加。
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		if ip := net.ParseIP(v); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
