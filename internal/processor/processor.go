package processor

import (
	"fmt"
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
	fmt.Printf("Cookie长度: %d\n", len(p.cfg.UserData.Cookie))

	doc, err := p.doRequest("GET", courseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("获取课程列表失败: %w", err)
	}

	// 调试：输出页面标题判断是否登录成功
	title := doc.Find("title").Text()
	fmt.Printf("页面标题: %s\n", title)

	var courseNames []string
	var totalItems int

	// 尝试多种选择器
	// 方式1: 直接查找 li.class-item
	doc.Find("li.class-item").Each(func(i int, s *goquery.Selection) {
		totalItems++
		// 获取状态
		status, _ := s.Attr("data-status")
		id, hasID := s.Attr("data-id")

		fmt.Printf("  课程 %d: status=%s, id=%s\n", i+1, status, id)

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
			fmt.Printf("    -> 添加课程: %s\n", name)
		}
	})

	// 如果方式1找不到，调试输出HTML结构
	if totalItems == 0 {
		fmt.Println("未找到 li.class-item，尝试查找其他元素...")
		// 输出页面中包含 class-item 的元素数量
		classItemCount := doc.Find("[class*='class-item']").Length()
		fmt.Printf("包含 'class-item' 的元素数量: %d\n", classItemCount)

		// 输出 ul 的数量
		ulCount := doc.Find("ul").Length()
		fmt.Printf("ul 元素数量: %d\n", ulCount)

		// 输出 li 的数量
		liCount := doc.Find("li").Length()
		fmt.Printf("li 元素数量: %d\n", liCount)
	}

	fmt.Printf("总共找到 %d 个课程项，其中 %d 个开放\n", totalItems, len(p.courseIDs))
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
			fmt.Printf("获取课程 %s 互动页面失败: %v\n", courseID, err)
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
		fmt.Printf("  找到题库: %s\n", quizName)
	})
}

// fetchQuizURLs 获取测验实际URL
func (p *DataProcessor) fetchQuizURLs() ([]QuizInfo, error) {
	var validQuizzes []QuizInfo

	for _, quiz := range p.quizList {
		time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

		doc, err := p.doRequest("GET", quiz.URL, baseURL)
		if err != nil {
			fmt.Printf("获取测验页面失败: %v\n", err)
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
		}
	}

	return validQuizzes, nil
}
