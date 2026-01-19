package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"mosoteach/internal/config"
	"mosoteach/internal/models"
	"mosoteach/internal/processor"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	loginURL = "https://www.mosoteach.cn/web/index.php?c=passport&m=index"

	// 时间常量
	loginWaitTime     = 10 * time.Second // 登录后等待时间
	pageLoadWaitTime  = 3 * time.Second  // 页面加载等待时间
	elementWaitTime   = 2 * time.Second  // 元素等待时间
	shortWaitTime     = 500 * time.Millisecond
	browserTimeout    = 30 * time.Minute  // 浏览器总超时
	apiRequestTimeout = 180 * time.Second // AI API 请求超时

	// 批量处理常量
	batchSize = 10 // 每批处理的题目数量
)

// QuestionType 题目类型
type QuestionType string

const (
	QuestionTypeFill     QuestionType = "填空题"
	QuestionTypeSingle   QuestionType = "单选题"
	QuestionTypeMultiple QuestionType = "多选题"
)

// Question 题目结构
type Question struct {
	Type    QuestionType
	Content string
	Options []Option
}

// Option 选项结构
type Option struct {
	Label string
	Text  string
}

// ProgressEvent 进度事件
type ProgressEvent struct {
	Type         string
	Message      string
	Progress     int    // 当前题目进度
	Total        int    // 题目总数
	QuizName     string // 当前题库名称
	QuizProgress int    // 当前题库进度（第几个）
	QuizTotal    int    // 题库总数
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(event ProgressEvent)

// BrowserExecutor 浏览器执行器
type BrowserExecutor struct {
	cfg           *config.Config
	modelManager  *models.ModelManager
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	ctx           context.Context
	cancel        context.CancelFunc
	timeoutCancel context.CancelFunc // 超时取消函数（独立保存）
	callback      ProgressCallback
}

// NewBrowserExecutor 创建浏览器执行器
func NewBrowserExecutor() *BrowserExecutor {
	cfg := config.GetConfig()
	return &BrowserExecutor{
		cfg:          cfg,
		modelManager: models.NewModelManager(),
	}
}

// NewBrowserExecutorWithCallback 创建带回调的浏览器执行器
func NewBrowserExecutorWithCallback(callback ProgressCallback) *BrowserExecutor {
	cfg := config.GetConfig()
	return &BrowserExecutor{
		cfg:          cfg,
		modelManager: models.NewModelManager(),
		callback:     callback,
	}
}

// sendProgress 发送进度事件
func (b *BrowserExecutor) sendProgress(eventType, message string, progress, total int) {
	b.sendFullProgress(eventType, message, progress, total, "", 0, 0)
}

// sendFullProgress 发送完整进度事件
func (b *BrowserExecutor) sendFullProgress(eventType, message string, progress, total int, quizName string, quizProgress, quizTotal int) {
	fmt.Println(message) // 同时打印到控制台
	if b.callback != nil {
		b.callback(ProgressEvent{
			Type:         eventType,
			Message:      message,
			Progress:     progress,
			Total:        total,
			QuizName:     quizName,
			QuizProgress: quizProgress,
			QuizTotal:    quizTotal,
		})
	}
}

// logDebug 调试日志，只打印到终端
func (b *BrowserExecutor) logDebug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("[DEBUG] " + msg)
}

// logInfo 信息日志，同时发送到前端
func (b *BrowserExecutor) logInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	b.sendProgress("log", msg, 0, 0)
}

// logf 格式化日志并同时发送到前端 (等同于 logInfo)
func (b *BrowserExecutor) logf(format string, args ...interface{}) {
	b.logInfo(format, args...)
}

// Start 启动浏览器
func (b *BrowserExecutor) Start() error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true), // 测试无头模式
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"),
	)

	// 设置Chrome路径
	if b.cfg.ChromeBinaryPath != "" {
		opts = append(opts, chromedp.ExecPath(b.cfg.ChromeBinaryPath))
	}

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	b.ctx, b.cancel = chromedp.NewContext(b.allocCtx)

	// 设置超时（保存超时取消函数，避免覆盖原始 cancel）
	b.ctx, b.timeoutCancel = context.WithTimeout(b.ctx, browserTimeout)

	return nil
}

// Stop 关闭浏览器
func (b *BrowserExecutor) Stop() {
	b.logDebug("正在关闭浏览器...")

	// 先取消超时 context
	if b.timeoutCancel != nil {
		b.timeoutCancel()
		b.timeoutCancel = nil
	}

	// 再取消 chromedp context（会关闭浏览器页面）
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}

	// 等待一下让浏览器有时间关闭
	time.Sleep(500 * time.Millisecond)

	// 最后取消 allocator（会杀掉 chrome 进程）
	if b.allocCancel != nil {
		b.allocCancel()
		b.allocCancel = nil
	}

	b.logDebug("浏览器已关闭")
}

// Login 登录并保存Cookie
func (b *BrowserExecutor) Login() error {
	b.logf("正在登录...")

	err := chromedp.Run(b.ctx,
		chromedp.Navigate(loginURL),
		chromedp.Sleep(elementWaitTime),
		chromedp.WaitVisible(`#account-name`, chromedp.ByID),
		chromedp.SendKeys(`#account-name`, b.cfg.UserData.UserName, chromedp.ByID),
		chromedp.Sleep(shortWaitTime),
		chromedp.SendKeys(`#user-pwd`, b.cfg.UserData.Password, chromedp.ByID),
		chromedp.Sleep(1*time.Second),
		chromedp.Click(`#login-button-1`, chromedp.ByID),
		chromedp.Sleep(loginWaitTime),
	)

	if err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}

	// 提取并保存Cookie（参照Python: driver.get_cookies()）
	if err := b.saveCookies(); err != nil {
		b.logf("警告: 保存Cookie失败: %v", err)
	}

	b.logf("登录成功!")
	return nil
}

// saveCookies 从浏览器提取Cookie并保存到配置文件
func (b *BrowserExecutor) saveCookies() error {
	var cookies []*network.Cookie
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)
	if err != nil {
		return fmt.Errorf("获取Cookie失败: %w", err)
	}

	// 转换为cookie字符串（参照Python: "; ".join([f"{cookie['name']}={cookie['value']}" for cookie in Cookies_list])）
	var parts []string
	for _, c := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	cookieStr := strings.Join(parts, "; ")

	// 保存到配置文件
	b.cfg.UserData.Cookie = cookieStr
	if err := b.cfg.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	b.logf("已保存 %d 个Cookie", len(cookies))
	return nil
}

// FetchQuizzesByBrowser 通过浏览器获取题库列表
func (b *BrowserExecutor) FetchQuizzesByBrowser() ([]processor.QuizInfo, error) {
	return b.FetchQuizzesByBrowserWithContext(context.Background())
}

// FetchQuizzesByBrowserWithContext 通过浏览器获取题库列表（带context）
func (b *BrowserExecutor) FetchQuizzesByBrowserWithContext(ctx context.Context) ([]processor.QuizInfo, error) {
	const courseURL = "https://www.mosoteach.cn/web/index.php?c=clazzcourse&m=index"

	// 检查是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	b.sendProgress("log", "正在获取课程列表...", 0, 0)

	// 导航到课程列表页面
	if err := chromedp.Run(b.ctx,
		chromedp.Navigate(courseURL),
		chromedp.Sleep(pageLoadWaitTime),
	); err != nil {
		return nil, fmt.Errorf("导航到课程页面失败: %w", err)
	}

	// 获取所有课程的ID和互动页面URL
	var courseData []map[string]string
	if err := chromedp.Run(b.ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('li.class-item')).map(li => ({
				id: li.getAttribute('data-id') || '',
				url: li.getAttribute('data-url') || '',
				status: li.getAttribute('data-status') || '',
				name: (li.querySelector('.class-info-subject') || {}).textContent || ''
			})).filter(c => c.status === 'OPEN')
		`, &courseData),
	); err != nil {
		return nil, fmt.Errorf("获取课程列表失败: %w", err)
	}

	b.logf("找到 %d 个开放课程", len(courseData))

	// 临时存储题库信息（带确认页URL）
	type tempQuizInfo struct {
		ConfirmURL string
		CourseID   string
		CourseName string
		QuizID     string
		Name       string
	}
	var tempQuizzes []tempQuizInfo
	seenQuizIDs := make(map[string]bool)

	// 遍历每个课程获取题库
	for i, course := range courseData {
		// 检查是否已取消
		select {
		case <-ctx.Done():
			b.sendProgress("log", "任务已取消", 0, 0)
			return nil, ctx.Err()
		default:
		}

		if course["url"] == "" {
			continue
		}

		courseName := strings.TrimSpace(course["name"])
		if courseName == "" {
			courseName = "未命名课程"
		}

		b.sendProgress("progress", fmt.Sprintf("正在获取课程 %d/%d: %s", i+1, len(courseData), courseName), i+1, len(courseData))

		// 导航到课程互动页面
		if err := chromedp.Run(b.ctx,
			chromedp.Navigate(course["url"]),
			chromedp.Sleep(elementWaitTime),
		); err != nil {
			b.logf("导航到课程 %s 失败: %v", courseName, err)
			continue
		}

		// 获取进行中的测验
		var quizData []map[string]string
		if err := chromedp.Run(b.ctx,
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('div.interaction-row')).map(row => ({
					id: row.getAttribute('data-id') || '',
					type: row.getAttribute('data-type') || '',
					status: row.getAttribute('data-row-status') || '',
					title: row.getAttribute('data-title') || ''
				})).filter(q => q.type === 'QUIZ' && q.status === 'IN_PRGRS')
			`, &quizData),
		); err != nil {
			b.logf("获取课程 %s 的题库失败: %v", courseName, err)
			continue
		}

		for _, quiz := range quizData {
			if quiz["id"] == "" {
				continue
			}

			// 检查是否已存在（去重）
			if seenQuizIDs[quiz["id"]] {
				b.logf("  跳过重复题库: %s", quiz["title"])
				continue
			}
			seenQuizIDs[quiz["id"]] = true

			confirmURL := fmt.Sprintf("https://www.mosoteach.cn/web/index.php?c=interaction_quiz&m=start_quiz_confirm&clazz_course_id=%s&id=%s&order_item=group",
				course["id"], quiz["id"])

			tempQuizzes = append(tempQuizzes, tempQuizInfo{
				ConfirmURL: confirmURL,
				CourseID:   course["id"],
				CourseName: courseName,
				QuizID:     quiz["id"],
				Name:       quiz["title"],
			})
			b.logf("  找到题库: %s (课程: %s)", quiz["title"], courseName)
		}
	}

	b.sendProgress("log", fmt.Sprintf("共找到 %d 个题库，正在获取答题链接...", len(tempQuizzes)), 0, 0)

	// 第二阶段：访问每个确认页面获取真正的答题URL
	var allQuizzes []processor.QuizInfo
	for i, tq := range tempQuizzes {
		// 检查是否已取消
		select {
		case <-ctx.Done():
			b.sendProgress("log", "任务已取消", 0, 0)
			return nil, ctx.Err()
		default:
		}

		b.sendProgress("progress", fmt.Sprintf("获取答题链接 %d/%d: %s", i+1, len(tempQuizzes), tq.Name), i+1, len(tempQuizzes))

		// 导航到确认页面
		if err := chromedp.Run(b.ctx,
			chromedp.Navigate(tq.ConfirmURL),
			chromedp.Sleep(elementWaitTime),
		); err != nil {
			b.logf("  导航到确认页面失败: %v", err)
			continue
		}

		// 获取隐藏的真正答题URL
		var hiddenURL string
		if err := chromedp.Run(b.ctx,
			chromedp.Evaluate(`
				(function() {
					var el = document.querySelector('div.hidden-box.hidden-url');
					return el ? el.textContent.trim() : '';
				})()
			`, &hiddenURL),
		); err != nil {
			b.logf("  获取答题URL失败: %v", err)
			continue
		}

		// 如果没找到，尝试从链接获取
		if hiddenURL == "" {
			var linkHref string
			if err := chromedp.Run(b.ctx,
				chromedp.Evaluate(`
					(function() {
						var a = document.querySelector('div.can-operate-color a');
						return a ? a.href : '';
					})()
				`, &linkHref),
			); err == nil && linkHref != "" {
				// 访问链接页面
				if err := chromedp.Run(b.ctx,
					chromedp.Navigate(linkHref),
					chromedp.Sleep(elementWaitTime),
				); err == nil {
					chromedp.Run(b.ctx,
						chromedp.Evaluate(`
							(function() {
								var el = document.querySelector('div.hidden-box.hidden-url');
								return el ? el.textContent.trim() : '';
							})()
						`, &hiddenURL),
					)
				}
			}
		}

		if hiddenURL == "" {
			b.logf("  未找到答题URL: %s", tq.Name)
			continue
		}

		allQuizzes = append(allQuizzes, processor.QuizInfo{
			URL:        hiddenURL,
			CourseID:   tq.CourseID,
			CourseName: tq.CourseName,
			QuizID:     tq.QuizID,
			Name:       tq.Name,
			Completed:  b.cfg.IsURLCompleted(hiddenURL),
		})
		b.logf("  获取答题URL成功: %s", tq.Name)
	}

	b.sendProgress("log", fmt.Sprintf("共获取 %d 个有效答题链接", len(allQuizzes)), 0, 0)
	return allQuizzes, nil
}

// processQuiz 处理单个测验（兼容旧调用）
func (b *BrowserExecutor) processQuiz(quiz processor.QuizInfo) error {
	return b.processQuizWithProgress(context.Background(), quiz, 1, 1)
}

// processQuizWithProgress 处理单个测验，带题库进度信息
func (b *BrowserExecutor) processQuizWithProgress(ctx context.Context, quiz processor.QuizInfo, quizProgress, quizTotal int) error {
	quizName := quiz.Name
	if quizName == "" {
		quizName = "未命名题库"
	}

	// 检查是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 重置进度条（重要：切换题库时必须重置）
	b.sendFullProgress("progress", fmt.Sprintf("正在加载: %s", quizName), 0, 0, quizName, quizProgress, quizTotal)

	// 导航到测验页面
	err := chromedp.Run(b.ctx,
		chromedp.Navigate(quiz.URL),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("加载测验页面失败: %w", err)
	}

	// 检查是否有"已用尽作答机会"的提示
	var pageHTML string
	chromedp.Run(b.ctx,
		chromedp.OuterHTML(`html`, &pageHTML, chromedp.ByQuery),
	)

	// 检测多种无法作答的情况
	if strings.Contains(pageHTML, "已用尽作答机会") ||
		strings.Contains(pageHTML, "未交卷") ||
		strings.Contains(pageHTML, "请联系老师") ||
		strings.Contains(pageHTML, "重新参与测试") ||
		strings.Contains(pageHTML, "pic_nothing") {
		b.logf("【%s】已用尽作答机会，跳过", quizName)
		// 标记为已完成，避免下次再尝试
		b.cfg.AddCompletedURL(quiz.URL)
		b.cfg.Save()
		return nil
	}

	// 检测空白页面（可能是无法作答的情况，JS未渲染完成）
	if strings.Contains(pageHTML, `class="blank"></div></div></div>`) ||
		strings.Contains(pageHTML, `<div class="blank"></div>`) {
		b.logf("【%s】页面为空白，可能无法作答，跳过", quizName)
		b.cfg.AddCompletedURL(quiz.URL)
		b.cfg.Save()
		return nil
	}

	// 等待题目容器加载，设置较短超时
	waitCtx, waitCancel := context.WithTimeout(b.ctx, loginWaitTime)
	defer waitCancel()

	err = chromedp.Run(waitCtx,
		chromedp.WaitVisible(`div.con-list`, chromedp.ByQuery),
	)
	if err != nil {
		chromedp.Run(b.ctx,
			chromedp.OuterHTML(`html`, &pageHTML, chromedp.ByQuery),
		)

		// 检测是否是无法作答的情况
		if strings.Contains(pageHTML, "已用尽作答机会") ||
			strings.Contains(pageHTML, "未交卷") ||
			strings.Contains(pageHTML, "请联系老师") ||
			strings.Contains(pageHTML, "重新参与测试") ||
			strings.Contains(pageHTML, "pic_nothing") ||
			strings.Contains(pageHTML, "m-disable") {
			b.logf("【%s】已用尽作答机会，跳过", quizName)
			b.cfg.AddCompletedURL(quiz.URL)
			b.cfg.Save()
			return nil
		}
		return fmt.Errorf("等待题目容器加载超时: %w", err)
	}

	// 获取页面HTML
	var htmlContent string
	err = chromedp.Run(b.ctx,
		chromedp.OuterHTML(`html`, &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("获取页面内容失败: %w", err)
	}

	// 解析题目
	questions, err := b.parseQuestions(htmlContent)
	if err != nil {
		return fmt.Errorf("解析题目失败: %w", err)
	}

	if len(questions) == 0 {
		b.sendFullProgress("log", "当前页面未找到题目", 0, 0, quizName, quizProgress, quizTotal)
		return nil
	}

	totalQuestions := len(questions)
	b.sendFullProgress("progress", fmt.Sprintf("【%s】共 %d 题，正在获取答案...", quizName, totalQuestions), 0, totalQuestions, quizName, quizProgress, quizTotal)

	// 批量获取所有题目的答案（一次API请求）
	answers, err := b.getBatchAnswers(ctx, questions, quizName, quizProgress, quizTotal)
	if err != nil {
		// 如果是取消错误，直接返回
		if ctx.Err() != nil {
			b.sendProgress("log", "任务已取消", 0, 0)
			return ctx.Err()
		}
		return fmt.Errorf("批量获取答案失败: %w", err)
	}

	// 检查是否已取消
	select {
	case <-ctx.Done():
		b.sendProgress("log", "任务已取消", 0, 0)
		return ctx.Err()
	default:
	}

	// 一次性批量填写所有答案（效率更高）
	b.sendFullProgress("progress", fmt.Sprintf("【%s】正在批量填写 %d 题...", quizName, totalQuestions), 0, totalQuestions, quizName, quizProgress, quizTotal)

	filledCount, err := b.batchSubmitAnswers(questions, answers)
	if err != nil {
		b.logf("批量填写出错: %v", err)
	}

	b.sendFullProgress("progress", fmt.Sprintf("【%s】%d 题已填写完毕，正在提交...", quizName, filledCount), totalQuestions, totalQuestions, quizName, quizProgress, quizTotal)

	// 提交整个测验
	return b.submitQuiz(quiz)
}

// parseQuestions 使用JavaScript在浏览器中直接获取题目信息（更可靠）
func (b *BrowserExecutor) parseQuestions(htmlContent string) ([]Question, error) {
	// 使用 JavaScript 直接获取题目信息，参照 Python 的 XPath 逻辑
	// Python XPath: //div[@class="t-con"]/div/div[@class="t-type SINGLE|MULTI|FILL"]
	jsGetQuestions := `
	(function() {
		var results = [];

		// 获取所有题型元素 - 使用精确的class匹配，与Python XPath一致
		// class="t-type SINGLE" 或 "t-type MULTI" 或 "t-type FILL"
		var typeElements = document.querySelectorAll('div[class="t-type SINGLE"], div[class="t-type MULTI"], div[class="t-type FILL"]');

		// 获取所有题干元素
		var stemElements = document.querySelectorAll('div.t-subject.t-item');

		// 获取所有选项区域
		var optionBlocks = document.querySelectorAll('div.t-option.t-item');

		console.log('找到题型元素: ' + typeElements.length);
		console.log('找到题干元素: ' + stemElements.length);
		console.log('找到选项区域: ' + optionBlocks.length);

		var count = Math.min(typeElements.length, stemElements.length);

		for (var i = 0; i < count; i++) {
			var typeEl = typeElements[i];
			var stemEl = stemElements[i];

			// 获取题型 - 从class属性中提取
			var typeCode = 'SINGLE';
			var classList = typeEl.className;
			if (classList.indexOf('MULTI') !== -1) {
				typeCode = 'MULTI';
			} else if (classList.indexOf('FILL') !== -1) {
				typeCode = 'FILL';
			}

			// 获取题干文本
			var stem = stemEl.innerText.trim();

			// 获取选项
			var options = [];
			if (i < optionBlocks.length && typeCode !== 'FILL') {
				var labels = optionBlocks[i].querySelectorAll('label.el-radio, label.el-checkbox');
				for (var j = 0; j < labels.length; j++) {
					var indexSpan = labels[j].querySelector('span.option-index');
					var contentSpan = labels[j].querySelector('span.option-content');
					if (indexSpan && contentSpan) {
						var label = indexSpan.innerText.trim().replace('.', '').replace(/\s/g, '');
						var text = contentSpan.innerText.trim();
						options.push({label: label, text: text});
					}
				}
			}

			results.push({
				type: typeCode,
				stem: stem,
				options: options
			});
		}

		return JSON.stringify(results);
	})()
	`

	var jsonResult string
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(jsGetQuestions, &jsonResult),
	)
	if err != nil {
		b.logDebug("JavaScript获取题目失败，回退到正则解析: %v", err)
		return b.parseQuestionsRegex(htmlContent)
	}

	b.logDebug("JavaScript返回结果长度: %d", len(jsonResult))

	// 解析 JSON 结果
	var jsQuestions []struct {
		Type    string `json:"type"`
		Stem    string `json:"stem"`
		Options []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
		} `json:"options"`
	}

	if err := json.Unmarshal([]byte(jsonResult), &jsQuestions); err != nil {
		b.logDebug("解析JavaScript结果失败，回退到正则解析: %v", err)
		return b.parseQuestionsRegex(htmlContent)
	}

	b.logDebug("JavaScript解析到 %d 道题", len(jsQuestions))

	if len(jsQuestions) == 0 {
		b.logDebug("JavaScript未找到题目，回退到正则解析")
		return b.parseQuestionsRegex(htmlContent)
	}

	var questions []Question
	singleCount, multiCount, fillCount := 0, 0, 0
	for i, jq := range jsQuestions {
		qType := QuestionTypeSingle
		switch jq.Type {
		case "MULTI":
			qType = QuestionTypeMultiple
			multiCount++
		case "FILL":
			qType = QuestionTypeFill
			fillCount++
		default:
			singleCount++
		}

		q := Question{
			Type:    qType,
			Content: jq.Stem,
		}

		for _, opt := range jq.Options {
			q.Options = append(q.Options, Option{
				Label: opt.Label,
				Text:  opt.Text,
			})
		}

		b.logDebug("题目 %d: %s, 选项数: %d", i+1, qType, len(q.Options))
		questions = append(questions, q)
	}

	b.logf("解析完成: 共 %d 题（单选 %d, 多选 %d, 填空 %d）", len(questions), singleCount, multiCount, fillCount)
	return questions, nil
}

// parseQuestionsRegex 使用正则表达式解析题目（备用方法）
func (b *BrowserExecutor) parseQuestionsRegex(htmlContent string) ([]Question, error) {
	var questions []Question

	// 直接用正则匹配所有题型，不依赖分割（因为HTML可能在一行里）
	typePattern := regexp.MustCompile(`<div class="t-type (SINGLE|MULTI|FILL)">`)
	typeMatches := typePattern.FindAllStringSubmatch(htmlContent, -1)

	// 匹配所有题干
	stemPattern := regexp.MustCompile(`<div class="t-subject t-item[^"]*"[^>]*>([^<]+)`)
	stemMatches := stemPattern.FindAllStringSubmatch(htmlContent, -1)

	b.logDebug("正则解析: 找到 %d 个题型, %d 个题干", len(typeMatches), len(stemMatches))

	// 匹配所有选项区域
	optionBlockPattern := regexp.MustCompile(`<div class="t-option t-item">([\s\S]*?)(?:<div class="t-upload|<div class="topic-item"|$)`)
	optionBlocks := optionBlockPattern.FindAllStringSubmatch(htmlContent, -1)

	count := len(typeMatches)
	if len(stemMatches) < count {
		count = len(stemMatches)
	}

	singleCount, multiCount, fillCount := 0, 0, 0
	for i := 0; i < count; i++ {
		qType := QuestionTypeSingle
		switch typeMatches[i][1] {
		case "MULTI":
			qType = QuestionTypeMultiple
			multiCount++
		case "FILL":
			qType = QuestionTypeFill
			fillCount++
		default:
			singleCount++
		}

		stem := cleanHTML(stemMatches[i][1])

		q := Question{
			Type:    qType,
			Content: stem,
		}

		// 解析选项
		if i < len(optionBlocks) && qType != QuestionTypeFill {
			optionPattern := regexp.MustCompile(`<span class="option-index">([A-Z])\.[^<]*</span>\s*<span class="option-content[^"]*"[^>]*>([^<]+)`)
			optMatches := optionPattern.FindAllStringSubmatch(optionBlocks[i][1], -1)
			for _, opt := range optMatches {
				q.Options = append(q.Options, Option{
					Label: opt[1],
					Text:  cleanHTML(opt[2]),
				})
			}
		}

		questions = append(questions, q)
	}

	b.logf("正则解析完成: 共 %d 题（单选 %d, 多选 %d, 填空 %d）", len(questions), singleCount, multiCount, fillCount)
	return questions, nil
}

// cleanHTML 清理HTML标签
func cleanHTML(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")

	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return text
}

// getAnswerWithContext 带context获取单个题目答案
func (b *BrowserExecutor) getAnswerWithContext(ctx context.Context, q Question) (string, error) {
	prompt := fmt.Sprintf("%s\n%s", string(q.Type), q.Content)

	for _, opt := range q.Options {
		prompt += fmt.Sprintf("\n%s.%s", opt.Label, opt.Text)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return b.modelManager.GetAnswer(reqCtx, prompt)
}

// getBatchAnswers 批量获取所有题目的答案（分批请求，每批最多10道题）
func (b *BrowserExecutor) getBatchAnswers(ctx context.Context, questions []Question, quizName string, quizProgress, quizTotal int) ([]string, error) {
	if len(questions) == 0 {
		return []string{}, nil
	}

	allAnswers := make([]string, len(questions))

	totalBatches := (len(questions) + batchSize - 1) / batchSize
	b.logf("共 %d 道题，分 %d 批处理", len(questions), totalBatches)

	for batchStart := 0; batchStart < len(questions); batchStart += batchSize {
		// 检查是否已取消
		select {
		case <-ctx.Done():
			b.logf("任务已取消，停止获取答案")
			return allAnswers, ctx.Err()
		default:
		}

		batchEnd := batchStart + batchSize
		if batchEnd > len(questions) {
			batchEnd = len(questions)
		}

		batchQuestions := questions[batchStart:batchEnd]
		batchNum := batchStart/batchSize + 1
		b.logf("正在处理第 %d/%d 批（题目 %d-%d）...", batchNum, totalBatches, batchStart+1, batchEnd)

		batchAnswers, err := b.getBatchAnswersForChunk(ctx, batchQuestions, batchStart)
		if err != nil {
			// 如果是取消错误，直接返回
			if ctx.Err() != nil {
				return allAnswers, ctx.Err()
			}
			b.logf("第 %d 批请求失败: %v，尝试逐题获取...", batchNum, err)
			// 失败时降级为逐题获取
			for i, q := range batchQuestions {
				// 检查取消
				select {
				case <-ctx.Done():
					return allAnswers, ctx.Err()
				default:
				}
				answer, err := b.getAnswerWithContext(ctx, q)
				if err != nil {
					if ctx.Err() != nil {
						return allAnswers, ctx.Err()
					}
					b.logf("第 %d 题获取失败: %v", batchStart+i+1, err)
					continue
				}
				allAnswers[batchStart+i] = answer
			}
			continue
		}

		// 将批次答案复制到总答案数组
		for i, ans := range batchAnswers {
			allAnswers[batchStart+i] = ans
			if ans != "" {
				b.logDebug("  → 第%d题答案: %s", batchStart+i+1, ans)
			} else {
				b.logDebug("  → 第%d题答案: (空)", batchStart+i+1)
			}
		}

		// 更新进度条
		b.sendFullProgress("progress", fmt.Sprintf("【%s】正在处理...", quizName), batchEnd, len(questions), quizName, quizProgress, quizTotal)

		// 批次之间稍作延迟，避免请求过快
		if batchEnd < len(questions) {
			time.Sleep(1 * time.Second)
		}
	}

	b.logf("批量获取完成，共 %d 道题", len(allAnswers))
	return allAnswers, nil
}

// getBatchAnswersForChunk 获取一批题目的答案
func (b *BrowserExecutor) getBatchAnswersForChunk(ctx context.Context, questions []Question, startIndex int) ([]string, error) {
	// 统计题目类型
	singleCount, multiCount, fillCount := 0, 0, 0
	for _, q := range questions {
		switch q.Type {
		case QuestionTypeSingle:
			singleCount++
		case QuestionTypeMultiple:
			multiCount++
		case QuestionTypeFill:
			fillCount++
		}
	}
	b.logDebug("本批题型统计: 单选%d, 多选%d, 填空%d (题目%d-%d)", singleCount, multiCount, fillCount, startIndex+1, startIndex+len(questions))

	// 构建批量提示词
	var promptBuilder strings.Builder
	promptBuilder.WriteString("请依次回答以下所有题目。每道题的答案用【答案X】标记，X是题号。\n")
	promptBuilder.WriteString("回答格式要求：\n")
	promptBuilder.WriteString("- 单选题：只回答选项字母，如 A\n")
	promptBuilder.WriteString("- 多选题：回答所有正确选项字母，用逗号分隔，如 A,B,C\n")
	promptBuilder.WriteString("- 填空题：直接回答答案内容\n\n")

	for i, q := range questions {
		promptBuilder.WriteString(fmt.Sprintf("【题目%d】%s\n", i+1, string(q.Type)))
		promptBuilder.WriteString(q.Content)
		promptBuilder.WriteString("\n")

		// 添加选项
		if len(q.Options) == 0 && q.Type != QuestionTypeFill {
			b.logDebug("警告: 第%d题（%s）没有选项！", i+1, q.Type)
		}
		for _, opt := range q.Options {
			promptBuilder.WriteString(fmt.Sprintf("%s.%s\n", opt.Label, opt.Text))
		}
		promptBuilder.WriteString("\n")
	}

	promptBuilder.WriteString("\n请按照格式回答所有题目：\n")
	for i := range questions {
		promptBuilder.WriteString(fmt.Sprintf("【答案%d】\n", i+1))
	}

	// 使用传入的 context，并添加超时
	reqCtx, cancel := context.WithTimeout(ctx, apiRequestTimeout) // 每批180秒超时
	defer cancel()

	response, err := b.modelManager.GetAnswer(reqCtx, promptBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("批量请求失败: %w", err)
	}

	// 调试：输出完整响应
	b.logDebug("=== API响应开始 ===\n%s\n=== API响应结束 ===", response)

	// 解析响应，提取每道题的答案
	answers := b.parseBatchAnswers(response, len(questions))

	return answers, nil
}

// parseBatchAnswers 解析批量回答，提取每道题的答案
func (b *BrowserExecutor) parseBatchAnswers(response string, questionCount int) []string {
	answers := make([]string, questionCount)

	// 尝试用【答案X】格式解析
	answerPattern := regexp.MustCompile(`【答案(\d+)】[：:\s]*([^\n【]+)`)
	matches := answerPattern.FindAllStringSubmatch(response, -1)

	b.logDebug("正则匹配到 %d 个答案", len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			idx := 0
			fmt.Sscanf(match[1], "%d", &idx)
			if idx >= 1 && idx <= questionCount {
				answer := strings.TrimSpace(match[2])
				// 清理答案格式
				answer = strings.ReplaceAll(answer, "。", "")
				answer = strings.ReplaceAll(answer, "，", ",")
				answer = strings.TrimSpace(answer)
				answers[idx-1] = answer
				b.logDebug("  解析到答案%d: %s", idx, answer)
			}
		}
	}

	// 如果没有匹配到足够的答案，尝试其他格式
	if countNonEmpty(answers) < questionCount {
		// 尝试用数字+点或数字+括号格式
		altPattern := regexp.MustCompile(`(?m)^(\d+)[.、)）]\s*([A-Za-z,，]+|[^\n]+)`)
		altMatches := altPattern.FindAllStringSubmatch(response, -1)

		for _, match := range altMatches {
			if len(match) >= 3 {
				idx := 0
				fmt.Sscanf(match[1], "%d", &idx)
				if idx >= 1 && idx <= questionCount && answers[idx-1] == "" {
					answer := strings.TrimSpace(match[2])
					answer = strings.ReplaceAll(answer, "，", ",")
					answers[idx-1] = answer
				}
			}
		}
	}

	return answers
}

// countNonEmpty 统计非空答案数量
func countNonEmpty(answers []string) int {
	count := 0
	for _, a := range answers {
		if a != "" {
			count++
		}
	}
	return count
}

// batchSubmitAnswers 一次性批量填写所有答案（高效模式）
func (b *BrowserExecutor) batchSubmitAnswers(questions []Question, answers []string) (int, error) {
	if len(questions) == 0 {
		return 0, nil
	}

	// 构建答案数据JSON
	type AnswerData struct {
		Index  int    `json:"index"`
		Type   string `json:"type"`
		Answer string `json:"answer"`
	}

	var answerList []AnswerData
	for i, q := range questions {
		answer := ""
		if i < len(answers) {
			answer = answers[i]
		}
		if answer == "" {
			continue
		}

		typeStr := "single"
		switch q.Type {
		case QuestionTypeMultiple:
			typeStr = "multi"
		case QuestionTypeFill:
			typeStr = "fill"
		}

		// 校正答案格式：单选题只取第一个选项
		if q.Type == QuestionTypeSingle && strings.Contains(answer, ",") {
			parts := strings.Split(answer, ",")
			answer = strings.TrimSpace(parts[0])
			b.logDebug("第%d题是单选但AI返回多选，已校正为: %s", i+1, answer)
		}

		// 调试：输出多选题的答案
		if q.Type == QuestionTypeMultiple {
			b.logDebug("第%d题是多选，答案: %s", i+1, answer)
		}

		answerList = append(answerList, AnswerData{
			Index:  i,
			Type:   typeStr,
			Answer: answer,
		})
	}

	// 将答案列表序列化为JSON
	answerJSON, err := json.Marshal(answerList)
	if err != nil {
		return 0, fmt.Errorf("序列化答案失败: %w", err)
	}

	b.logDebug("批量填写 %d 个答案", len(answerList))

	// 使用异步 JavaScript 脚本一次性填写所有答案，确保每次点击有足够时间响应
	jsBatchFill := fmt.Sprintf(`
		(async function() {
			var answers = %s;
			var filledCount = 0;
			var debugLog = [];
			var subjects = document.querySelectorAll('.t-subject.t-item');
			var fillInputs = document.querySelectorAll('.tp-blank input.el-input__inner');

			// 延迟函数
			function sleep(ms) {
				return new Promise(resolve => setTimeout(resolve, ms));
			}

			debugLog.push('subjects数量: ' + subjects.length);

			for (var a = 0; a < answers.length; a++) {
				var item = answers[a];
				var idx = item.index;
				var type = item.type;
				var answer = item.answer;

				try {
					if (type === 'fill') {
						// 填空题
						if (idx < fillInputs.length) {
							var input = fillInputs[idx];
							input.value = answer;
							input.dispatchEvent(new Event('input', { bubbles: true }));
							input.dispatchEvent(new Event('change', { bubbles: true }));
							filledCount++;
						}
					} else {
						// 选择题（单选/多选）
						if (idx < subjects.length) {
							var subject = subjects[idx];
							var optionDiv = subject.parentElement.querySelector('.t-option');
							if (!optionDiv) {
								debugLog.push('题目' + (idx+1) + ': 未找到optionDiv');
								continue;
							}

							var labels = optionDiv.querySelectorAll('label.el-radio, label.el-checkbox');

							// 答案可能是 "A,B,C" 形式
							var answerLetters = answer.replace(/\s/g, '').split(',');

							if (type === 'multi') {
								debugLog.push('多选题' + (idx+1) + ': 找到' + labels.length + '个选项, 需点击[' + answerLetters.join(',') + ']');
							}

							// 用于记录需要点击的元素
							var elementsToClick = [];

							for (var i = 0; i < labels.length; i++) {
								// 尝试多种方式获取选项字母
								var optionLetter = '';

								// 方式1: 查找 span.option-index
								var indexSpan = labels[i].querySelector('span.option-index');
								if (indexSpan) {
									optionLetter = indexSpan.textContent.trim().charAt(0).toUpperCase();
								}

								// 方式2: 查找 span:nth-child(2) > div > span:first-child
								if (!optionLetter) {
									var span2 = labels[i].querySelector('span:nth-child(2)');
									if (span2) {
										var innerSpan = span2.querySelector('div > span:first-child');
										if (innerSpan) {
											optionLetter = innerSpan.textContent.trim().charAt(0).toUpperCase();
										}
									}
								}

								// 方式3: 直接查找所有 span 并找到包含选项字母的
								if (!optionLetter) {
									var allSpans = labels[i].querySelectorAll('span');
									for (var s = 0; s < allSpans.length; s++) {
										var text = allSpans[s].textContent.trim();
										if (/^[A-Z][.．。]/.test(text)) {
											optionLetter = text.charAt(0).toUpperCase();
											break;
										}
									}
								}

								if (optionLetter) {
									for (var j = 0; j < answerLetters.length; j++) {
										if (optionLetter === answerLetters[j].toUpperCase()) {
											elementsToClick.push({label: labels[i], letter: optionLetter});
											break;
										}
									}
								}
							}

							if (type === 'multi') {
								debugLog.push('  找到需点击的选项: ' + elementsToClick.map(e => e.letter).join(','));
							}

							// 逐个点击，每次点击后等待一下让Vue响应
							for (var k = 0; k < elementsToClick.length; k++) {
								var elem = elementsToClick[k];
								if (type === 'multi') {
									debugLog.push('  -> 正在点击选项 ' + elem.letter);
								}

								// 滚动到元素可见
								elem.label.scrollIntoView({block: 'center'});

								// 尝试点击 input 元素（Element UI checkbox 的实际可点击元素）
								var inputElem = elem.label.querySelector('input');
								if (inputElem) {
									inputElem.click();
								} else {
									elem.label.click();
								}

								filledCount++;

								// 多选题时，每次点击后等待一下让Vue响应
								if (type === 'multi' && k < elementsToClick.length - 1) {
									await sleep(100);
								}
							}
						}
					}
				} catch(e) {
					debugLog.push('题目' + idx + '出错: ' + e.message);
				}
			}

			return {count: filledCount, log: debugLog.join('|')};
		})()
	`, string(answerJSON))

	var resultMap map[string]interface{}
	err = chromedp.Run(b.ctx,
		chromedp.EvaluateAsDevTools(jsBatchFill, &resultMap),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return 0, fmt.Errorf("执行批量填写失败: %w", err)
	}

	// 从 map 中提取结果
	var count int
	var log string
	if c, ok := resultMap["count"].(float64); ok {
		count = int(c)
	}
	if l, ok := resultMap["log"].(string); ok {
		log = l
	}

	// 输出 JavaScript 调试日志
	if log != "" {
		for _, line := range strings.Split(log, "|") {
			if line != "" {
				b.logDebug("JS: %s", line)
			}
		}
	}

	b.logDebug("批量填写完成")
	return count, nil
}

// submitQuiz 提交测验
func (b *BrowserExecutor) submitQuiz(quiz processor.QuizInfo) error {
	// 检查是否需要延迟提交
	delay := b.cfg.GetSubmitDelay()
	if delay > 0 {
		b.logf("等待 %d 秒后提交...", delay)
		for elapsed := 1; elapsed <= delay; elapsed++ {
			time.Sleep(1 * time.Second)
			b.sendProgress("submit_countdown", quiz.URL, elapsed, delay)
			remaining := delay - elapsed
			if remaining > 0 {
				b.logf("距离提交还有 %d 秒", remaining)
			}
		}
		b.logf("延迟等待结束，开始提交")
	}

	b.logf("正在提交测验...")

	var clicked bool
	err := chromedp.Run(b.ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			(function() {
				// 方法1: 直接通过class选择器
				var btn = document.querySelector('.con-bottom button.el-button--primary');
				if (btn) {
					btn.click();
					return true;
				}
				// 方法2: 通过文字内容查找
				var buttons = document.querySelectorAll('button.el-button');
				for (var i = 0; i < buttons.length; i++) {
					if (buttons[i].textContent.includes('交卷')) {
						buttons[i].click();
						return true;
					}
				}
				return false;
			})()
		`, &clicked),
	)
	if err != nil {
		b.logDebug("点击交卷按钮失败: %v", err)
	}
	if clicked {
		b.logDebug("成功点击交卷按钮")
	} else {
		b.logDebug("未找到交卷按钮，尝试备用方法")
	}

	err = chromedp.Run(b.ctx,
		chromedp.Sleep(1500*time.Millisecond),
	)

	// 点击 el-message-box 中的确认按钮
	var confirmClicked bool
	err = chromedp.Run(b.ctx,
		chromedp.Evaluate(`
			(function() {
				// 查找 el-message-box 对话框
				var msgBox = document.querySelector('.el-message-box');
				if (msgBox) {
					// 在对话框中查找确认按钮（通常是 primary 类型或包含"确定"文字）
					var btns = msgBox.querySelectorAll('.el-message-box__btns button, .el-button');
					for (var i = 0; i < btns.length; i++) {
						var btn = btns[i];
						// 优先点击 primary 按钮或包含"确定/确认"的按钮
						if (btn.classList.contains('el-button--primary') ||
							btn.textContent.includes('确定') ||
							btn.textContent.includes('确认')) {
							btn.click();
							return true;
						}
					}
					// 如果没找到，点击最后一个按钮（通常是确认）
					if (btns.length > 0) {
						btns[btns.length - 1].click();
						return true;
					}
				}
				return false;
			})()
		`, &confirmClicked),
	)
	if confirmClicked {
		b.logDebug("成功点击确认按钮")
	} else {
		b.logDebug("未找到确认对话框，尝试备用方法")
		// 备用：尝试点击任何可见的 primary 按钮
		chromedp.Run(b.ctx,
			chromedp.Evaluate(`
				var btn = document.querySelector('.el-message-box .el-button--primary');
				if (btn) btn.click();
			`, nil),
		)
	}

	// 步骤3: 等待可能的第二次确认
	chromedp.Run(b.ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			// 再次检查是否有对话框需要确认
			var msgBox = document.querySelector('.el-message-box');
			if (msgBox) {
				var btn = msgBox.querySelector('.el-button--primary, button');
				if (btn) btn.click();
			}
		`, nil),
		chromedp.Sleep(3*time.Second),
	)

	// 记录已完成的URL
	b.cfg.AddCompletedURL(quiz.URL)
	b.cfg.MarkQuizCompleted(quiz.URL)
	b.cfg.Save()

	b.sendProgress("quiz_completed", quiz.URL, 0, 0)
	b.logf("测验提交成功!")

	return nil
}

// Run 运行自动答题
func (b *BrowserExecutor) Run() error {
	return b.RunWithContext(context.Background())
}

// RunWithContext 带context运行自动答题
func (b *BrowserExecutor) RunWithContext(ctx context.Context) error {
	// 启动浏览器
	if err := b.Start(); err != nil {
		return err
	}
	defer b.Stop()

	b.sendProgress("log", "正在登录...", 0, 0)

	// 登录（会自动保存Cookie）
	if err := b.Login(); err != nil {
		return err
	}

	// 重新加载配置（获取新Cookie）
	b.cfg.Load()

	b.sendProgress("log", "正在获取题库列表...", 0, 0)

	// 获取待处理的测验（现在使用新的Cookie）
	proc, err := processor.NewDataProcessor()
	if err != nil {
		return fmt.Errorf("创建数据处理器失败: %w", err)
	}

	quizzes, err := proc.FetchPendingQuizzes()
	if err != nil {
		return fmt.Errorf("获取测验列表失败: %w", err)
	}

	// 只处理未完成的
	var pendingQuizzes []processor.QuizInfo
	for _, q := range quizzes {
		if !q.Completed {
			pendingQuizzes = append(pendingQuizzes, q)
		}
	}

	// 处理测验
	return b.ProcessQuizzesWithContext(ctx, pendingQuizzes)
}

// RunSingleQuiz 运行单个题库
func (b *BrowserExecutor) RunSingleQuiz(ctx context.Context, quizURL string) error {
	return b.RunMultipleQuizzes(ctx, []string{quizURL})
}

// RunMultipleQuizzes 运行多个指定题库
func (b *BrowserExecutor) RunMultipleQuizzes(ctx context.Context, quizURLs []string) error {
	// 启动浏览器
	if err := b.Start(); err != nil {
		return err
	}
	defer b.Stop()

	b.sendProgress("log", "正在登录...", 0, 0)

	// 登录
	if err := b.Login(); err != nil {
		return err
	}

	// 重新加载配置
	b.cfg.Load()

	// 构建题库列表
	quizzes := make([]processor.QuizInfo, len(quizURLs))
	for i, url := range quizURLs {
		quizzes[i] = processor.QuizInfo{
			URL:  url,
			Name: fmt.Sprintf("选中题库 %d", i+1),
		}
	}

	return b.ProcessQuizzesWithContext(ctx, quizzes)
}

// ProcessQuizzesWithContext 带context处理测验
func (b *BrowserExecutor) ProcessQuizzesWithContext(ctx context.Context, quizzes []processor.QuizInfo) error {
	quizTotal := len(quizzes)
	if quizTotal == 0 {
		b.sendProgress("log", "当前已无测试题可做", 0, 0)
		return nil
	}

	b.sendFullProgress("progress", fmt.Sprintf("共有 %d 个题库待处理", quizTotal), 0, 0, "", 0, quizTotal)

	for i, quiz := range quizzes {
		// 检查是否取消
		select {
		case <-ctx.Done():
			b.sendProgress("log", "任务已取消", 0, 0)
			return ctx.Err()
		default:
		}

		quizName := quiz.Name
		if quizName == "" {
			quizName = fmt.Sprintf("题库 %d", i+1)
		}

		b.sendFullProgress("progress", fmt.Sprintf("正在处理: %s (%d/%d)", quizName, i+1, quizTotal), 0, 0, quizName, i+1, quizTotal)

		if err := b.processQuizWithProgress(ctx, quiz, i+1, quizTotal); err != nil {
			// 如果是取消错误，直接返回不继续处理
			if ctx.Err() != nil {
				b.sendProgress("log", "任务已取消", 0, 0)
				return ctx.Err()
			}
			b.sendProgress("log", fmt.Sprintf("处理失败: %v", err), 0, 0)
			continue
		}

		time.Sleep(2 * time.Second)
	}

	b.sendFullProgress("complete", "已完成所有题库", 0, 0, "", quizTotal, quizTotal)
	return nil
}
