package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
)

// mediaServer 本地 HTTP 服务，按 projectID 暴露媒体文件
type mediaServer struct {
	mu      sync.RWMutex
	paths   map[string]string // projectID → 媒体文件绝对路径
	server  *http.Server
	baseURL string
}

// NewMediaServer 创建并启动本地媒体服务（监听随机端口）
func NewMediaServer() (*mediaServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("media server listen: %w", err)
	}

	ms := &mediaServer{
		paths: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/media/", ms.handleMedia)

	ms.server = &http.Server{Handler: mux}
	ms.baseURL = "http://" + ln.Addr().String()

	go func() {
		_ = ms.server.Serve(ln)
	}()

	return ms, nil
}

// Register 注册项目媒体文件路径
func (ms *mediaServer) Register(projectID, mediaPath string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.paths[projectID] = mediaPath
}

// URL 返回项目媒体文件的访问 URL
func (ms *mediaServer) URL(projectID string) string {
	return fmt.Sprintf("%s/media/%s", ms.baseURL, projectID)
}

// Shutdown 关闭服务
func (ms *mediaServer) Shutdown() {
	_ = ms.server.Shutdown(context.Background())
}

// handleMedia 处理 /media/<projectID> 请求，支持 Range（视频 seek）
func (ms *mediaServer) handleMedia(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Path[len("/media/"):]
	if projectID == "" {
		http.Error(w, "missing project id", http.StatusBadRequest)
		return
	}

	ms.mu.RLock()
	path, ok := ms.paths[projectID]
	ms.mu.RUnlock()
	if !ok {
		http.Error(w, "project media not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, path)
}

// 防止未使用导入（当 ServeFile 不直接用 io 时保留）
var _ = io.EOF
var _ = strconv.Itoa
