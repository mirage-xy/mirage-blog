// Package web 是 HTTP 层：路由、中间件、handler。
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// shutdownTimeout 是优雅退出的上限：等在途请求做完，超时就强关。
const shutdownTimeout = 5 * time.Second

// Server 持有 HTTP 服务。生命周期放在 Serve 里而不是 main 里，测试才能自己起 listener 验证退出。
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	logger     *slog.Logger
}

// New 组装路由，返回一个还没开始监听的 Server。
func New(addr string, logger *slog.Logger) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		logger: logger,
	}
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.routes()
	return s
}

// routes 注册路由。通配路由要放在具体路由后面，见《开发规范》§4.1。
func (s *Server) routes() {
	s.mux.Handle("GET /healthz", chain(http.HandlerFunc(s.handleHealthz), s.recoverMW, s.logMW))
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		s.logger.Error("write healthz response", "err", err)
	}
}

// Serve 在 ln 上提供服务。ctx 取消后停止接收新请求，等在途请求做完再返回。
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.httpServer.Serve(ln) }()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	s.logger.Info("server stopped")
	return nil
}
