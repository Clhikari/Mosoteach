package processor

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"mosoteach/internal/config"
)

const (
	baseURL         = "https://www.mosoteach.cn"
	courseURL       = "https://www.mosoteach.cn/web/index.php?c=clazzcourse&m=index"
	interactionURL  = "https://www.mosoteach.cn/web/index.php?c=interaction&m=index"
	quizConfirmPre  = "https://www.mosoteach.cn/web/index.php?c=interaction_quiz&m=start_quiz_confirm&clazz_course_id="
	userAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
)

// QuizInfo 测试信息
type QuizInfo struct {
	URL        string
	CourseID   string
	CourseName string // 课程名称
	QuizID     string
	Name       string // 题库名称
	Completed  bool   // 是否已完成
}

// CourseInfo 课程信息（带题库）
type CourseInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Quizzes   []QuizInfo `json:"quizzes"`
}

// DataProcessor 数据处理器
type DataProcessor struct {
	cfg         *config.Config
	client      *http.Client
	courseIDs   []string
	courseNames []string
	quizList    []QuizInfo
}

// NewDataProcessor 创建数据处理器
func NewDataProcessor() (*DataProcessor, error) {
	cfg := config.GetConfig()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("创建cookie jar失败: %w", err)
	}

	// 解析并设置cookie
	baseU, _ := url.Parse(baseURL)
	cookies := parseCookies(cfg.UserData.Cookie)
	jar.SetCookies(baseU, cookies)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	return &DataProcessor{
		cfg:         cfg,
		client:      client,
		courseIDs:   make([]string, 0),
		courseNames: make([]string, 0),
		quizList:    make([]QuizInfo, 0),
	}, nil
}

// parseCookies 解析cookie字符串
func parseCookies(cookieStr string) []*http.Cookie {
	var cookies []*http.Cookie
	pairs := strings.Split(cookieStr, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			cookies = append(cookies, &http.Cookie{
				Name:  strings.TrimSpace(parts[0]),
				Value: strings.TrimSpace(parts[1]),
			})
		}
	}
	return cookies
}

// doRequest 发送HTTP请求
func (p *DataProcessor) doRequest(method, reqURL, referer string) (*goquery.Document, error) {
	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

// FetchCourseList 获取课程列表
func (p *DataProcessor) FetchCourseList() ([]string, error) {
	// 检查Cookie是否存在
	if p.cfg.UserData.Cookie == "" {
		return nil, fmt.Errorf("Cookie为空，请先运行一次答题任务以获取登录Cookie")
	}

	doc, err := p.doRequest("GET", courseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("获取课程列表失败: %w", err)
	}

	var courseNames []string
	var totalItems int

	// 尝试多种选择器
	// 方式1: 直接查找 li.class-item
	doc.Find("li.class-item").Each(func(i int, s *goquery.Selection) {
		totalItems++
		// 获取状态
		status, _ := s.Attr("data-status")
		id, hasID := s.Attr("data-id")

		// 只获取开放的课程 (data-status="OPEN")
		if status != "OPEN" {
			return
		}

		// 获取课程ID
		if hasID && id != "" {
			p.courseIDs = append(p.courseIDs, id)

			// 获取课程名称 (class-info-subject)
			name := strings.TrimSpace(s.Find("span.class-info-subject").Text())
			if name == "" {
				// 尝试其他选择器
				name = strings.TrimSpace(s.Find(".class-info-subject").Text())
			}
			if name == "" {
				name = "未命名课程"
			}
			courseNames = append(courseNames, name)
			slog.Debug("找到课程", "name", name, "id", id)
		}
	})

	slog.Debug("课程统计", "total", totalItems, "open", len(p.courseIDs))
	p.courseNames = courseNames
	return courseNames, nil
}

// FetchPendingQuizzes 获取待完成的测验
func (p *DataProcessor) FetchPendingQuizzes() ([]QuizInfo, error) {
	if len(p.courseIDs) == 0 {
		_, err := p.FetchCourseList()
		if err != nil {
			return nil, err
		}
	}

	for i, courseID := range p.courseIDs {
		// 随机延迟
		time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

		interactURL := interactionURL + "&clazz_course_id=" + courseID
		doc, err := p.doRequest("GET", interactURL, baseURL)
		if err != nil {
			continue
		}

		// 解析互动活动
		p.parseInteractions(doc, courseID, i)
	}

	// 获取测验URL
	return p.fetchQuizURLs()
}

// parseInteractions 解析互动活动
func (p *DataProcessor) parseInteractions(doc *goquery.Document, courseID string, index int) {
	// 查找所有互动行
	doc.Find("div.interaction-row").Each(func(i int, row *goquery.Selection) {
		// 检查是否是测验类型 (data-type="QUIZ")
		dataType, _ := row.Attr("data-type")
		if dataType != "QUIZ" {
			return
		}

		// 检查是否是进行中 (data-row-status="IN_PRGRS")
		rowStatus, _ := row.Attr("data-row-status")
		if rowStatus != "IN_PRGRS" {
			return
		}

		// 获取题库ID
		quizID, exists := row.Attr("data-id")
		if !exists || quizID == "" {
			return
		}

		// 获取题库名称 (优先从data-title获取，否则从span.interaction-name)
		quizName, _ := row.Attr("data-title")
		if quizName == "" {
			quizName = strings.TrimSpace(row.Find("span.interaction-name").Text())
		}
		if quizName == "" {
			quizName = "未命名题库"
		}

		quizURL := quizConfirmPre + courseID + "&id=" + quizID + "&order_item=group"
		p.quizList = append(p.quizList, QuizInfo{
			URL:      quizURL,
			CourseID: courseID,
			QuizID:   quizID,
			Name:     quizName,
		})
		slog.Debug("找到测验", "name", quizName, "quiz_id", quizID, "course_id", courseID)
	})
}

// fetchQuizURLs 获取测验实际URL
func (p *DataProcessor) fetchQuizURLs() ([]QuizInfo, error) {
	var validQuizzes []QuizInfo

	for _, quiz := range p.quizList {
		time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

		doc, err := p.doRequest("GET", quiz.URL, baseURL)
		if err != nil {
			continue
		}

		// 获取隐藏的URL
		hiddenURL := strings.TrimSpace(doc.Find("div.hidden-box.hidden-url").Text())

		if hiddenURL == "" {
			// 尝试从链接获取
			if link, exists := doc.Find("div.can-operate-color a").Attr("href"); exists {
				subDoc, err := p.doRequest("GET", link, baseURL)
				if err == nil {
					hiddenURL = strings.TrimSpace(subDoc.Find("div.hidden-box.hidden-url").Text())
				}
			}
		}

		if hiddenURL != "" {
			completed := p.cfg.IsURLCompleted(hiddenURL)
			validQuizzes = append(validQuizzes, QuizInfo{
				URL:       hiddenURL,
				CourseID:  quiz.CourseID,
				QuizID:    quiz.QuizID,
				Name:      quiz.Name,
				Completed: completed,
			})
			slog.Debug("获取测验URL", "name", quiz.Name, "completed", completed)
		}
	}

	slog.Debug("测验统计", "total", len(p.quizList), "valid", len(validQuizzes))
	return validQuizzes, nil
}
