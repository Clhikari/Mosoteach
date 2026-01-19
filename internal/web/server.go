package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"mosoteach/internal/browser"
	"mosoteach/internal/config"
	"mosoteach/internal/models"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFiles embed.FS

// ProgressEvent 进度事件
type ProgressEvent struct {
	Type         string `json:"type"` // log, progress, complete, error
	Message      string `json:"message"`
	Progress     int    `json:"progress"`               // 当前题目进度
	Total        int    `json:"total"`                  // 题目总数
	QuizName     string `json:"quizName,omitempty"`     // 当前题库名称
	QuizProgress int    `json:"quizProgress,omitempty"` // 当前题库进度
	QuizTotal    int    `json:"quizTotal,omitempty"`    // 题库总数
}

// Server Web服务器
type Server struct {
	mu         sync.RWMutex
	cfg        *config.Config
	executor   *browser.BrowserExecutor
	status     *Status
	sseClients map[chan ProgressEvent]bool
	sseMu      sync.RWMutex
	cancelFunc context.CancelFunc
	sessions   map[string]time.Time // 会话令牌 -> 过期时间
	sessionMu  sync.RWMutex
}

// Status 当前状态
type Status struct {
	Running     bool   `json:"running"`
	Message     string `json:"message"`
	Progress    int    `json:"progress"`
	Total       int    `json:"total"`
	CurrentTask string `json:"currentTask"`
}

// NewServer 创建服务器
func NewServer() *Server {
	return &Server{
		cfg: config.GetConfig(),
		status: &Status{
			Running: false,
			Message: "就绪",
		},
		sseClients: make(map[chan ProgressEvent]bool),
		sessions:   make(map[string]time.Time),
	}
}

// Start 启动服务器
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// API路由
	mux.HandleFunc("/api/auth/check", s.handleAuthCheck)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/save", s.handleSaveConfig)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/models/save", s.handleSaveModels)
	mux.HandleFunc("/api/models/test", s.handleTestModel)
	mux.HandleFunc("/api/quizzes", s.handleQuizzes)
	mux.HandleFunc("/api/quizzes/cache", s.handleQuizzesCache)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/events", s.handleSSE)
	mux.HandleFunc("/api/settings/submit-delay", s.handleSubmitDelay)
	mux.HandleFunc("/api/settings/web-password", s.handleWebPassword)

	// 静态文件服务
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🚀 服务器已启动: http://localhost%s\n", addr)

	// 使用 Basic Auth 中间件包装
	return http.ListenAndServe(addr, s.authMiddleware(mux))
}

// authMiddleware Cookie 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password := s.cfg.GetWebPassword()

		// 如果没设置密码，直接放行
		if password == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 放行认证相关的 API 和静态资源
		if strings.HasPrefix(r.URL.Path, "/api/auth/") || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		// 静态资源放行
		if strings.HasPrefix(r.URL.Path, "/css/") || strings.HasSuffix(r.URL.Path, ".ico") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查 Cookie
		cookie, err := r.Cookie("mosoteach_auth")
		if err != nil || !s.validateSession(cookie.Value) {
			// API 请求返回 401
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// 页面请求重定向到首页（前端会显示登录界面）
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateSessionToken 生成随机会话令牌
func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// createSession 创建新会话
func (s *Server) createSession() (string, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", err
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	// 会话有效期 7 天
	s.sessions[token] = time.Now().Add(7 * 24 * time.Hour)
	return token, nil
}

// validateSession 验证会话
func (s *Server) validateSession(token string) bool {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	expiry, exists := s.sessions[token]
	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		// 会话已过期，删除
		go func() {
			s.sessionMu.Lock()
			delete(s.sessions, token)
			s.sessionMu.Unlock()
		}()
		return false
	}
	return true
}

// clearSessions 清除所有会话（密码修改时调用）
func (s *Server) clearSessions() {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.sessions = make(map[string]time.Time)
}

// handleAuthCheck 检查认证状态
func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	password := s.cfg.GetWebPassword()

	// 没设置密码，不需要认证
	if password == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"required": false,
			"authenticated": true,
		})
		return
	}

	// 检查 Cookie
	cookie, err := r.Cookie("mosoteach_auth")
	authenticated := err == nil && s.validateSession(cookie.Value)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"required": true,
		"authenticated": authenticated,
	})
}

// handleAuthLogin 处理登录
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if s.cfg.VerifyWebPassword(req.Password) {
		// 创建新会话
		token, err := s.createSession()
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		// 设置认证 Cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "mosoteach_auth",
			Value:    token,
			Path:     "/",
			MaxAge:   86400 * 7, // 7 天
			HttpOnly: true,
			Secure:   true, // 仅在 HTTPS 下发送
			SameSite: http.SameSiteLaxMode,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "密码错误",
	})
}

// handleConfig 获取配置
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.cfg.Load()

	// 返回用户配置
	response := map[string]interface{}{
		"user_name":    s.cfg.UserData.UserName,
		"has_password": s.cfg.UserData.Password != "",
		"has_cookie":   s.cfg.UserData.Cookie != "",
		"masked_user":  s.cfg.GetMaskedUsername(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSaveConfig 保存用户配置
func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserName string `json:"user_name"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if req.UserName != "" {
		s.cfg.UserData.UserName = req.UserName
	}
	if req.Password != "" {
		s.cfg.UserData.SetPassword(req.Password)
	}
	s.mu.Unlock()

	if err := s.cfg.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "配置保存成功"})
}

// handleModels 获取模型配置
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.cfg.Load()

	// 返回模型列表（隐藏API Key明文）
	models := make([]map[string]interface{}, len(s.cfg.Models))
	for i, m := range s.cfg.Models {
		models[i] = map[string]interface{}{
			"name":        m.Name,
			"enabled":     m.Enabled,
			"base_url":    m.BaseURL,
			"model":       m.Model,
			"has_api_key": m.APIKey != "",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

// handleSaveModels 保存模型配置
func (s *Server) handleSaveModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var models []config.ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&models); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 合并API Key（如果新配置中为空则保留原有的）
	s.mu.Lock()
	for i := range models {
		if models[i].APIKey == "" {
			// 查找原有模型的API Key
			for _, oldModel := range s.cfg.Models {
				if oldModel.Name == models[i].Name {
					models[i].APIKey = oldModel.APIKey
					break
				}
			}
		}
	}
	s.mu.Unlock()

	if err := s.cfg.UpdateModels(models); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "模型配置保存成功"})
}

// handleTestModel 测试模型是否可用
func (s *Server) handleTestModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req config.ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 如果没有传API Key，尝试从已保存的配置中获取
	if req.APIKey == "" {
		s.cfg.Load()
		for _, m := range s.cfg.Models {
			if m.Name == req.Name {
				req.APIKey = m.APIKey
				break
			}
		}
	}

	// 验证必要字段
	if req.BaseURL == "" || req.Model == "" || req.APIKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请填写完整的配置（Base URL、模型名称、API Key）",
		})
		return
	}

	// 创建模型并测试
	model := models.NewUnifiedModel(req)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	answer, err := model.GetAnswer(ctx, "请回复：测试成功")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("连接失败: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "连接成功",
		"reply":   answer,
	})
}

// handleStart 开始答题
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求参数
	var req struct {
		QuizURL  string   `json:"quizUrl"`  // 可选：指定单个题库URL（兼容旧版）
		QuizURLs []string `json:"quizUrls"` // 可选：指定多个题库URL
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		// 忽略空 body 的情况
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "任务正在运行中",
		})
		return
	}
	s.status.Running = true
	s.status.Message = "正在初始化..."
	s.status.Progress = 0
	s.mu.Unlock()

	// 创建可取消的context
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	// 异步执行答题
	go func() {
		defer func() {
			s.mu.Lock()
			s.status.Running = false
			s.cancelFunc = nil
			s.executor = nil
			s.mu.Unlock()
		}()

		s.sendSSEEvent(ProgressEvent{Type: "log", Message: "正在启动浏览器..."})

		executor := browser.NewBrowserExecutorWithCallback(s.progressCallback)
		s.mu.Lock()
		s.executor = executor
		s.mu.Unlock()

		var err error
		if len(req.QuizURLs) > 0 {
			// 答多个选中的题库
			err = executor.RunMultipleQuizzes(ctx, req.QuizURLs)
		} else if req.QuizURL != "" {
			// 答单个指定题库（兼容旧版）
			err = executor.RunSingleQuiz(ctx, req.QuizURL)
		} else {
			// 答所有题库
			err = executor.RunWithContext(ctx)
		}

		if err != nil {
			// 区分取消和真正的错误
			if ctx.Err() != nil {
				// 用户取消 - 发送cancelled事件并重置进度
				s.sendSSEEvent(ProgressEvent{Type: "cancelled", Message: "任务已取消", Progress: 0, Total: 0})
				s.mu.Lock()
				s.status.Message = "任务已取消"
				s.status.Progress = 0
				s.status.Total = 0
				s.mu.Unlock()
			} else {
				// 真正的错误
				s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("错误: %v", err)})
				s.mu.Lock()
				s.status.Message = fmt.Sprintf("错误: %v", err)
				s.mu.Unlock()
			}
			return
		}

		s.sendSSEEvent(ProgressEvent{Type: "complete", Message: "已完成所有题目"})
		s.mu.Lock()
		s.status.Message = "已完成所有题目"
		s.status.Progress = s.status.Total
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "任务已启动",
	})
}

// progressCallback 进度回调
func (s *Server) progressCallback(event browser.ProgressEvent) {
	s.mu.Lock()
	s.status.Message = event.Message
	if event.Total > 0 {
		s.status.Total = event.Total
	}
	if event.Progress > 0 {
		s.status.Progress = event.Progress
	}
	s.status.CurrentTask = event.Message
	s.mu.Unlock()

	// 转换为web包的ProgressEvent
	s.sendSSEEvent(ProgressEvent{
		Type:         event.Type,
		Message:      event.Message,
		Progress:     event.Progress,
		Total:        event.Total,
		QuizName:     event.QuizName,
		QuizProgress: event.QuizProgress,
		QuizTotal:    event.QuizTotal,
	})
}

// handleStop 停止答题
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	// 显式关闭浏览器进程
	if s.executor != nil {
		s.executor.Stop()
		s.executor = nil
	}
	s.status.Running = false
	s.status.Message = "已停止"
	s.mu.Unlock()

	s.sendSSEEvent(ProgressEvent{Type: "log", Message: "任务已停止"})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// handleStatus 获取状态
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	status := *s.status
	s.mu.RUnlock()

	// 如果不在运行中，动态检查就绪状态
	if !status.Running {
		status.Message = s.checkReadyStatus()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// checkReadyStatus 检查系统就绪状态
func (s *Server) checkReadyStatus() string {
	s.cfg.Load()
	_, message := s.cfg.IsReady()
	return message
}

// handleQuizzes 获取题库列表（使用浏览器）
func (s *Server) handleQuizzes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "有任务正在运行中，请稍后再试",
		})
		return
	}
	s.status.Running = true
	s.status.Message = "正在获取题库..."
	s.status.Progress = 0
	s.status.Total = 0
	s.mu.Unlock()

	// 创建可取消的context
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	s.sendSSEEvent(ProgressEvent{Type: "log", Message: "正在启动浏览器获取题库列表..."})

	// 使用浏览器获取题库
	executor := browser.NewBrowserExecutorWithCallback(s.progressCallback)
	defer executor.Stop()

	if err := executor.Start(); err != nil {
		s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("启动浏览器失败: %v", err)})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 先登录
	if err := executor.Login(); err != nil {
		s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("登录失败: %v", err)})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取题库（使用可取消的context）
	quizzes, err := executor.FetchQuizzesByBrowserWithContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			s.sendSSEEvent(ProgressEvent{Type: "log", Message: "获取题库已取消"})
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("获取题库失败: %v", err)})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 保存到缓存
	cachedQuizzes := make([]config.CachedQuiz, len(quizzes))
	for i, q := range quizzes {
		cachedQuizzes[i] = config.CachedQuiz{
			URL:        q.URL,
			CourseID:   q.CourseID,
			CourseName: q.CourseName,
			QuizID:     q.QuizID,
			Name:       q.Name,
			Completed:  q.Completed,
		}
	}
	s.cfg.SaveCachedQuizzes(cachedQuizzes)

	// 转换为JSON友好的格式
	type QuizResponse struct {
		URL        string `json:"url"`
		Name       string `json:"name"`
		CourseID   string `json:"courseId"`
		CourseName string `json:"courseName"`
		QuizID     string `json:"quizId"`
		Completed  bool   `json:"completed"`
	}

	var response []QuizResponse
	for _, q := range quizzes {
		response = append(response, QuizResponse{
			URL:        q.URL,
			Name:       q.Name,
			CourseID:   q.CourseID,
			CourseName: q.CourseName,
			QuizID:     q.QuizID,
			Completed:  q.Completed,
		})
	}

	s.sendSSEEvent(ProgressEvent{Type: "log", Message: fmt.Sprintf("找到 %d 个题库", len(response))})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQuizzesCache 获取缓存的题库列表
func (s *Server) handleQuizzesCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cachedQuizzes := s.cfg.GetCachedQuizzes()

	type QuizResponse struct {
		URL        string `json:"url"`
		Name       string `json:"name"`
		CourseID   string `json:"courseId"`
		CourseName string `json:"courseName"`
		QuizID     string `json:"quizId"`
		Completed  bool   `json:"completed"`
	}

	var response []QuizResponse
	for _, q := range cachedQuizzes {
		response = append(response, QuizResponse{
			URL:        q.URL,
			Name:       q.Name,
			CourseID:   q.CourseID,
			CourseName: q.CourseName,
			QuizID:     q.QuizID,
			Completed:  q.Completed,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSSE SSE事件流
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建客户端通道
	clientChan := make(chan ProgressEvent, 100)

	// 注册客户端
	s.sseMu.Lock()
	s.sseClients[clientChan] = true
	s.sseMu.Unlock()

	// 清理函数
	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, clientChan)
		close(clientChan)
		s.sseMu.Unlock()
	}()

	// 发送初始连接事件
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"SSE连接成功\"}\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// 监听事件
	for {
		select {
		case event, ok := <-clientChan:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

// sendSSEEvent 向所有SSE客户端发送事件
func (s *Server) sendSSEEvent(event ProgressEvent) {
	s.sseMu.RLock()
	defer s.sseMu.RUnlock()

	for clientChan := range s.sseClients {
		select {
		case clientChan <- event:
		default:
			// 通道满了，跳过
		}
	}
}

// handleLogin 处理登录请求（刷新Cookie）
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "有任务正在运行中",
		})
		return
	}
	s.status.Running = true
	s.status.Message = "正在登录..."
	s.mu.Unlock()

	// 异步执行登录
	go func() {
		defer func() {
			s.mu.Lock()
			s.status.Running = false
			s.mu.Unlock()
		}()

		s.sendSSEEvent(ProgressEvent{Type: "log", Message: "正在启动浏览器登录..."})

		executor := browser.NewBrowserExecutor()
		defer executor.Stop()

		// 先启动浏览器
		if err := executor.Start(); err != nil {
			s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("启动浏览器失败: %v", err)})
			s.mu.Lock()
			s.status.Message = fmt.Sprintf("启动浏览器失败: %v", err)
			s.mu.Unlock()
			return
		}

		if err := executor.Login(); err != nil {
			s.sendSSEEvent(ProgressEvent{Type: "error", Message: fmt.Sprintf("登录失败: %v", err)})
			s.mu.Lock()
			s.status.Message = fmt.Sprintf("登录失败: %v", err)
			s.mu.Unlock()
			return
		}

		s.sendSSEEvent(ProgressEvent{Type: "complete", Message: "登录成功，Cookie已更新"})
		s.mu.Lock()
		s.status.Message = "登录成功"
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "正在登录...",
	})
}

// handleSubmitDelay 处理提交延迟配置
func (s *Server) handleSubmitDelay(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		delay := s.cfg.GetSubmitDelay()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"submit_delay": delay})

	case http.MethodPost:
		var req struct {
			SubmitDelay int `json:"submit_delay"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if req.SubmitDelay < 0 {
			req.SubmitDelay = 0
		}
		if err := s.cfg.SetSubmitDelay(req.SubmitDelay); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"submit_delay": req.SubmitDelay,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWebPassword 处理 Web 访问密码配置
func (s *Server) handleWebPassword(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		password := s.cfg.GetWebPassword()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"has_password": password != "",
		})

	case http.MethodPost:
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.cfg.SetWebPassword(req.Password); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg := "密码已设置"
		if req.Password == "" {
			msg = "密码已清除"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": msg,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
