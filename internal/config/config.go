package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ModelConfig 模型配置
type ModelConfig struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// UserData 用户配置
type UserData struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
	Cookie   string `json:"Cookie"`
}

// GetPassword 获取密码
func (u *UserData) GetPassword() string {
	return u.Password
}

// SetPassword 设置密码
func (u *UserData) SetPassword(password string) {
	u.Password = password
}

// HasPassword 检查是否有密码
func (u *UserData) HasPassword() bool {
	return u.Password != ""
}

// CachedQuiz 缓存的题库
type CachedQuiz struct {
	URL        string `json:"url"`
	CourseID   string `json:"course_id"`
	CourseName string `json:"course_name"`
	QuizID     string `json:"quiz_id"`
	Name       string `json:"name"`
	Completed  bool   `json:"completed"`
}

// ConfigFile 配置文件结构
type ConfigFile struct {
	UserData      UserData      `json:"user_data"`
	Models        []ModelConfig `json:"models"`
	CachedQuizzes []CachedQuiz  `json:"cached_quizzes,omitempty"`
	CompletedURLs []string      `json:"completed_urls,omitempty"`
}

// Config 全局配置管理
type Config struct {
	mu               sync.RWMutex
	UserData         UserData
	Models           []ModelConfig
	CachedQuizzes    []CachedQuiz
	FilePath         string
	ChromeBinaryPath string
	IsLinux          bool
	CompletedURLs    map[string]bool
}

var (
	instance *Config
	once     sync.Once
)

// GetConfig 获取配置单例
func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{
			CompletedURLs: make(map[string]bool),
			Models:        getDefaultModels(),
		}
		instance.initPaths()
	})
	return instance
}

// getDefaultModels 获取默认模型配置
func getDefaultModels() []ModelConfig {
	return []ModelConfig{
		{
			Name:    "DeepSeek",
			Enabled: true,
			BaseURL: "https://api.deepseek.com",
			APIKey:  "",
			Model:   "deepseek-chat",
		},
		{
			Name:    "Gemini",
			Enabled: false,
			BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			APIKey:  "",
			Model:   "gemini-2.5-flash",
		},
		{
			Name:    "OpenAI",
			Enabled: false,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "",
			Model:   "gpt-4o",
		},
		{
			Name:    "通义千问",
			Enabled: false,
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			APIKey:  "",
			Model:   "qwen-plus",
		},
		{
			Name:    "Moonshot",
			Enabled: false,
			BaseURL: "https://api.moonshot.cn/v1",
			APIKey:  "",
			Model:   "moonshot-v1-auto",
		},
		{
			Name:    "Ollama",
			Enabled: false,
			BaseURL: "http://localhost:11434/v1",
			APIKey:  "ollama",
			Model:   "qwen3:8b",
		},
	}
}

// initPaths 初始化路径配置
func (c *Config) initPaths() {
	c.IsLinux = runtime.GOOS == "linux"
	c.FilePath = "./user_data.json"

	if c.IsLinux {
		c.ChromeBinaryPath = c.findChromeBinary()
	} else {
		c.ChromeBinaryPath = c.findWindowsChrome()
	}
}

// findWindowsChrome Windows 下自动查找 Chrome 二进制文件
func (c *Config) findWindowsChrome() string {
	paths := []string{
		// 常见安装路径
		os.Getenv("PROGRAMFILES") + "\\Google\\Chrome\\Application\\chrome.exe",
		os.Getenv("PROGRAMFILES(X86)") + "\\Google\\Chrome\\Application\\chrome.exe",
		os.Getenv("LOCALAPPDATA") + "\\Google\\Chrome\\Application\\chrome.exe",
		// 便携版常见路径
		".\\chrome-win64\\chrome.exe",
		"..\\chrome-win64\\chrome.exe",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// findChromeBinary Linux下自动查找Chrome
func (c *Config) findChromeBinary() string {
	binaries := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/snap/bin/chromium",
	}

	for _, binary := range binaries {
		if _, err := os.Stat(binary); err == nil {
			return binary
		}
	}
	return ""
}

// Load 加载配置文件
func (c *Config) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.FilePath)
	if err != nil {
		// 文件不存在，使用默认配置
		if os.IsNotExist(err) {
			c.Models = getDefaultModels()
			return c.saveInternal()
		}
		return err
	}

	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return err
	}

	c.UserData = configFile.UserData
	c.CachedQuizzes = configFile.CachedQuizzes

	// 如果配置文件中有模型配置则使用，否则使用默认
	if len(configFile.Models) > 0 {
		c.Models = configFile.Models
	} else {
		c.Models = getDefaultModels()
	}

	// 加载已完成的URL列表
	for _, url := range configFile.CompletedURLs {
		c.CompletedURLs[url] = true
	}

	return nil
}

// Save 保存配置文件
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.saveInternal()
}

// saveInternal 内部保存方法（不加锁）
func (c *Config) saveInternal() error {
	// 将 map 转换为 slice
	var completedURLs []string
	for url := range c.CompletedURLs {
		completedURLs = append(completedURLs, url)
	}

	configFile := ConfigFile{
		UserData:      c.UserData,
		Models:        c.Models,
		CachedQuizzes: c.CachedQuizzes,
		CompletedURLs: completedURLs,
	}

	data, err := json.MarshalIndent(configFile, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.FilePath, data, 0644)
}

// UpdateCookie 更新Cookie
func (c *Config) UpdateCookie(cookie string) error {
	c.mu.Lock()
	c.UserData.Cookie = cookie
	c.mu.Unlock()
	return c.Save()
}

// AddCompletedURL 添加已完成的URL
func (c *Config) AddCompletedURL(url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.CompletedURLs[url] {
		return nil
	}

	c.CompletedURLs[url] = true
	return c.saveInternal()
}

// IsURLCompleted 检查URL是否已完成
func (c *Config) IsURLCompleted(url string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CompletedURLs[url]
}

// GetMaskedUsername 获取脱敏用户名
func (c *Config) GetMaskedUsername() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	username := c.UserData.UserName
	if len(username) == 11 {
		return username[:3] + "****" + username[7:]
	}
	return username
}

// GetAbsPath 获取绝对路径
func (c *Config) GetAbsPath(relativePath string) string {
	absPath, err := filepath.Abs(relativePath)
	if err != nil {
		return relativePath
	}
	return absPath
}

// GetEnabledModels 获取已启用的模型列表
func (c *Config) GetEnabledModels() []ModelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var enabled []ModelConfig
	for _, m := range c.Models {
		if m.Enabled && m.APIKey != "" {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// UpdateModels 更新模型配置
func (c *Config) UpdateModels(models []ModelConfig) error {
	c.mu.Lock()
	c.Models = models
	c.mu.Unlock()
	return c.Save()
}

// AddModel 添加新模型
func (c *Config) AddModel(model ModelConfig) error {
	c.mu.Lock()
	c.Models = append(c.Models, model)
	c.mu.Unlock()
	return c.Save()
}

// GetCachedQuizzes 获取缓存的题库
func (c *Config) GetCachedQuizzes() []CachedQuiz {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 更新完成状态
	result := make([]CachedQuiz, len(c.CachedQuizzes))
	for i, q := range c.CachedQuizzes {
		result[i] = q
		result[i].Completed = c.CompletedURLs[q.URL]
	}
	return result
}

// SaveCachedQuizzes 保存缓存的题库
func (c *Config) SaveCachedQuizzes(quizzes []CachedQuiz) error {
	c.mu.Lock()
	c.CachedQuizzes = quizzes
	c.mu.Unlock()
	return c.Save()
}

// MarkQuizCompleted 标记题库为已完成
func (c *Config) MarkQuizCompleted(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CompletedURLs[url] = true
	// 更新缓存中的状态
	for i := range c.CachedQuizzes {
		if c.CachedQuizzes[i].URL == url {
			c.CachedQuizzes[i].Completed = true
			break
		}
	}
}

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ValidateUserData 验证用户数据配置
func (c *Config) ValidateUserData() []ValidationError {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var errors []ValidationError

	if c.UserData.UserName == "" {
		errors = append(errors, ValidationError{
			Field:   "user_name",
			Message: "用户名不能为空",
		})
	} else if len(c.UserData.UserName) != 11 {
		errors = append(errors, ValidationError{
			Field:   "user_name",
			Message: "用户名应为11位手机号",
		})
	}

	if c.UserData.Password == "" {
		errors = append(errors, ValidationError{
			Field:   "password",
			Message: "密码不能为空",
		})
	}

	return errors
}

// ValidateModels 验证模型配置
func (c *Config) ValidateModels() []ValidationError {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var errors []ValidationError

	hasEnabled := false
	for i, m := range c.Models {
		if m.Enabled {
			hasEnabled = true
			if m.APIKey == "" {
				errors = append(errors, ValidationError{
					Field:   "models[" + string(rune('0'+i)) + "].api_key",
					Message: "已启用的模型 " + m.Name + " 缺少 API Key",
				})
			}
			if m.BaseURL == "" {
				errors = append(errors, ValidationError{
					Field:   "models[" + string(rune('0'+i)) + "].base_url",
					Message: "已启用的模型 " + m.Name + " 缺少 Base URL",
				})
			}
			if m.Model == "" {
				errors = append(errors, ValidationError{
					Field:   "models[" + string(rune('0'+i)) + "].model",
					Message: "已启用的模型 " + m.Name + " 缺少模型名称",
				})
			}
		}
	}

	if !hasEnabled {
		errors = append(errors, ValidationError{
			Field:   "models",
			Message: "至少需要启用一个模型",
		})
	}

	return errors
}

// Validate 验证所有配置
func (c *Config) Validate() []ValidationError {
	var errors []ValidationError
	errors = append(errors, c.ValidateUserData()...)
	errors = append(errors, c.ValidateModels()...)
	return errors
}

// IsReady 检查配置是否就绪（可以开始答题）
func (c *Config) IsReady() (bool, string) {
	userErrors := c.ValidateUserData()
	modelErrors := c.ValidateModels()

	if len(userErrors) > 0 && len(modelErrors) > 0 {
		return false, "请配置账号和模型"
	}
	if len(userErrors) > 0 {
		return false, userErrors[0].Message
	}
	if len(modelErrors) > 0 {
		return false, modelErrors[0].Message
	}

	return true, "就绪"
}
