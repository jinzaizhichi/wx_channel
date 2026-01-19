package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/public"

	"wx_channel/internal/api"
	"wx_channel/internal/assets"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/handlers"
	"wx_channel/internal/storage"
	"wx_channel/internal/utils"
	"wx_channel/internal/websocket"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

// App structure to hold dependencies and state
type App struct {
	Sunny          *SunnyNet.Sunny
	Cfg            *config.Config
	Version        string
	Port           int
	CurrentPageURL string
	LogInitMsg     string

	// Managers
	CSVManager  *storage.CSVManager
	FileManager *storage.FileManager

	// Handlers
	APIHandler        *handlers.APIHandler
	UploadHandler     *handlers.UploadHandler
	RecordHandler     *handlers.RecordHandler
	ScriptHandler     *handlers.ScriptHandler
	BatchHandler      *handlers.BatchHandler
	CommentHandler    *handlers.CommentHandler
	ConsoleAPIHandler *handlers.ConsoleAPIHandler
	WebSocketHandler  *handlers.WebSocketHandler

	// Services
	WSHub         *websocket.Hub
	SearchService *api.SearchService
}

// Global variable to bridge SunnyNet C-style callback to App method
var globalApp *App

// NewApp creates and initializes a new App instance
func NewApp(cfgParam *config.Config) *App {
	app := &App{
		Sunny:   SunnyNet.NewSunny(),
		Cfg:     cfgParam,
		Version: "?t=" + cfgParam.Version,
		Port:    cfgParam.Port,
	}

	// Set global instance for callback bridge
	globalApp = app

	// Initialize Logging
	utils.LogConfigLoad("config.yaml", true)
	if app.Cfg.LogFile != "" {
		_ = utils.InitLoggerWithRotation(utils.INFO, app.Cfg.LogFile, app.Cfg.MaxLogSizeMB)
		app.LogInitMsg = fmt.Sprintf("日志已初始化: %s (最大 %dMB)", app.Cfg.LogFile, app.Cfg.MaxLogSizeMB)
	}

	return app
}

// downloadRecordsHeader CSV 文件的表头
var downloadRecordsHeader = []string{"ID", "标题", "视频号名称", "视频号分类", "公众号名称", "视频链接", "页面链接", "文件大小", "时长", "阅读量", "点赞量", "评论量", "收藏数", "转发数", "创建时间", "IP所在地", "下载时间", "页面来源", "搜索关键词"}

// initDownloadRecords 初始化下载记录系统
func (app *App) initDownloadRecords() error {
	downloadsDir, err := utils.ResolveDownloadDir(app.Cfg.DownloadsDir)
	if err != nil {
		return fmt.Errorf("解析下载目录失败: %v", err)
	}

	app.FileManager, err = storage.NewFileManager(downloadsDir)
	if err != nil {
		return fmt.Errorf("创建文件管理器失败: %v", err)
	}

	csvPath := filepath.Join(downloadsDir, app.Cfg.RecordsFile)
	app.CSVManager, err = storage.NewCSVManager(csvPath, downloadRecordsHeader)
	if err != nil {
		return fmt.Errorf("创建CSV管理器失败: %v", err)
	}

	return nil
}

// Run 启动应用
func (app *App) Run() {
	os_env := runtime.GOOS

	// 确保端口设置正确
	app.Sunny.SetPort(app.Port)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		color.Red("\n正在关闭服务...%v\n\n", sig)
		utils.LogSystemShutdown(fmt.Sprintf("收到信号: %v", sig))
		database.Close()
		if os_env == "darwin" {
			proxy.DisableProxyInMacOS(proxy.ProxySettings{
				Device:   "",
				Hostname: "127.0.0.1",
				Port:     strconv.Itoa(app.Port),
			})
		}
		os.Exit(0)
	}()

	app.printTitle()

	if err := app.initDownloadRecords(); err != nil {
		utils.HandleError(err, "初始化下载记录系统")
	} else {
		app.printDownloadRecordInfo()
		if app.LogInitMsg != "" {
			utils.Info(app.LogInitMsg)
			app.LogInitMsg = ""
		}
	}

	app.printEnvConfig()

	// Initialize Handlers
	app.APIHandler = handlers.NewAPIHandler(app.Cfg)

	if app.CSVManager != nil {
		app.UploadHandler = handlers.NewUploadHandler(app.Cfg, app.CSVManager)
		app.RecordHandler = handlers.NewRecordHandler(app.Cfg, app.CSVManager)
	}

	// 使用 assets 包中的资源
	app.ScriptHandler = handlers.NewScriptHandler(app.Cfg, assets.CoreJS, assets.DecryptJS, assets.DownloadJS, assets.HomeJS, assets.FeedJS, assets.ProfileJS, assets.SearchJS, assets.BatchDownloadJS, assets.ZipJS, assets.FileSaverJS, assets.MittJS, assets.EventbusJS, assets.UtilsJS, assets.APIClientJS, app.Version)

	if app.CSVManager != nil {
		app.BatchHandler = handlers.NewBatchHandler(app.Cfg, app.CSVManager)
	}

	app.CommentHandler = handlers.NewCommentHandler(app.Cfg)

	downloadsDir, err := utils.ResolveDownloadDir(app.Cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录用于数据库初始化")
	} else {
		dbPath := filepath.Join(downloadsDir, "console.db")
		if err := database.Initialize(&database.Config{DBPath: dbPath}); err != nil {
			utils.HandleError(err, "初始化数据库")
			utils.Warn("Web控制台功能可能受限")
		} else {
			utils.Info("✓ 数据库已初始化: %s", dbPath)
			settingsRepo := database.NewSettingsRepository()
			config.SetDatabaseLoader(settingsRepo)

			// 重新加载配置
			app.Cfg = config.Reload()
			utils.Info("✓ 配置已从数据库重新加载")

			// Update port if changed in DB (implementation detail: sunny net might need restart if port changes mid-flight, but for now we follow old logic)

			if err := app.initDownloadRecords(); err != nil {
				utils.HandleError(err, "重新初始化下载记录系统")
			} else {
				utils.Info("✓ 下载记录系统已使用新配置重新初始化")
				if app.CSVManager != nil {
					app.UploadHandler = handlers.NewUploadHandler(app.Cfg, app.CSVManager)
					app.RecordHandler = handlers.NewRecordHandler(app.Cfg, app.CSVManager)
					app.BatchHandler = handlers.NewBatchHandler(app.Cfg, app.CSVManager)
					utils.Info("✓ 处理器已使用新配置重新初始化")
				}
			}
		}
	}

	app.ConsoleAPIHandler = handlers.NewConsoleAPIHandler(app.Cfg)
	app.WebSocketHandler = handlers.NewWebSocketHandler()

	existing, err1 := certificate.CheckCertificate("SunnyNet")
	if err1 != nil {
		utils.HandleError(err1, "检查证书")
		utils.Warn("程序将继续运行，但HTTPS功能可能受限...")
		existing = false
	} else if !existing {
		utils.Info("正在安装证书...")
		err := certificate.InstallCertificate(assets.CertData)
		time.Sleep(app.Cfg.CertInstallDelay)
		if err != nil {
			utils.HandleError(err, "证书安装")
			utils.Warn("如需完整功能，请手动安装证书或以管理员身份运行程序。")

			if app.FileManager != nil {
				downloadsDir, err := utils.ResolveDownloadDir(app.Cfg.DownloadsDir)
				if err == nil {
					certPath := filepath.Join(downloadsDir, app.Cfg.CertFile)
					if err := utils.EnsureDir(downloadsDir); err == nil {
						if err := os.WriteFile(certPath, assets.CertData, 0644); err == nil {
							utils.Info("证书文件已保存到: %s", certPath)
						}
					}
				}
			}
		} else {
			utils.Info("✓ 证书安装成功！")
		}
	} else {
		utils.Info("✓ 证书已存在，无需重新安装。")
	}

	app.Sunny.SetGoCallback(GlobalHttpCallback, nil, nil, nil)
	sunnyErr := app.Sunny.Start().Error
	if sunnyErr != nil {
		utils.HandleError(sunnyErr, "启动代理服务")
		utils.Warn("按 Ctrl+C 退出...")
		select {}
	}

	proxy_server := fmt.Sprintf("127.0.0.1:%v", app.Port)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   proxy_server,
			}),
		},
	}
	_, err3 := client.Get("https://sunny.io/")
	if err3 == nil {
		if os_env == "windows" {
			ok := app.Sunny.StartProcess()
			if !ok {
				color.Red("\nERROR 启动进程代理失败，检查是否以管理员身份运行\n")
				color.Yellow("按 Ctrl+C 退出...\n")
				select {}
			}
			app.Sunny.ProcessAddName("WeChatAppEx.exe")
		}

		utils.PrintSeparator()
		color.Blue("📡 服务状态信息")
		utils.PrintSeparator()
		utils.PrintLabelValue("⏳", "服务状态", "已启动")
		utils.PrintLabelValue("🔌", "代理端口", app.Port)
		utils.PrintLabelValue("📱", "支持平台", "微信视频号")

		proxyMode := "进程代理"
		if os_env != "windows" {
			proxyMode = "系统代理"
		}
		utils.LogSystemStart(app.Port, proxyMode)

		app.WSHub = websocket.NewHub()
		go app.WSHub.Run()
		app.SearchService = api.NewSearchService(app.WSHub)
		utils.Info("✓ WebSocket Hub 已初始化")

		wsPort := app.Port + 1
		go app.startWebSocketServer(wsPort)

		utils.Info("🔍 请打开需要下载的视频号页面进行下载")
	} else {
		utils.PrintSeparator()
		utils.Warn("⚠️ 您还未安装证书，请在浏览器打开 http://%v 并根据说明安装证书", proxy_server)
		utils.Warn("⚠️ 在安装完成后重新启动此程序即可")
		utils.PrintSeparator()
	}
	utils.Info("💡 服务正在运行，按 Ctrl+C 退出...")
	select {}
}

// GlobalHttpCallback bridges to the singleton app instance
func GlobalHttpCallback(Conn *SunnyNet.HttpConn) {
	if globalApp != nil {
		globalApp.HandleRequest(Conn)
	}
}

// HandleRequest 处理 HTTP 回调
func (app *App) HandleRequest(Conn *SunnyNet.HttpConn) {
	host := Conn.Request.URL.Hostname()
	path := Conn.Request.URL.Path
	if Conn.Type == public.HttpSendRequest {
		Conn.Request.Header.Del("Accept-Encoding")

		// 使用 assets 中的资源
		if handlers.HandleStaticFiles(Conn, assets.ZipJS, assets.FileSaverJS) {
			return
		}

		if app.APIHandler != nil {
			if app.APIHandler.HandleProfile(Conn) {
				return
			}
			if app.APIHandler.HandleTip(Conn) {
				return
			}
			if app.APIHandler.HandlePageURL(Conn) {
				app.CurrentPageURL = app.APIHandler.GetCurrentURL()
				if app.RecordHandler != nil {
					app.RecordHandler.SetCurrentURL(app.CurrentPageURL)
				}
				return
			}
		}

		if app.UploadHandler != nil {
			if app.UploadHandler.HandleInitUpload(Conn) {
				return
			}
			if app.UploadHandler.HandleUploadChunk(Conn) {
				return
			}
			if app.UploadHandler.HandleCompleteUpload(Conn) {
				return
			}
			if app.UploadHandler.HandleUploadStatus(Conn) {
				return
			}
			if app.UploadHandler.HandleSaveVideo(Conn) {
				return
			}
			if app.UploadHandler.HandleSaveCover(Conn) {
				return
			}
			if app.UploadHandler.HandleDownloadVideo(Conn) {
				return
			}
		}

		if app.RecordHandler != nil {
			if app.RecordHandler.HandleRecordDownload(Conn) {
				return
			}
			if app.RecordHandler.HandleExportVideoList(Conn) {
				return
			}
			if app.RecordHandler.HandleExportVideoListJSON(Conn) {
				return
			}
			if app.RecordHandler.HandleExportVideoListMarkdown(Conn) {
				return
			}
			if app.RecordHandler.HandleBatchDownloadStatus(Conn) {
				return
			}
		}

		if app.BatchHandler != nil {
			if app.BatchHandler.HandleBatchStart(Conn) {
				return
			}
			if app.BatchHandler.HandleBatchProgress(Conn) {
				return
			}
			if app.BatchHandler.HandleBatchCancel(Conn) {
				return
			}
			if app.BatchHandler.HandleBatchResume(Conn) {
				return
			}
			if app.BatchHandler.HandleBatchClear(Conn) {
				return
			}
			if app.BatchHandler.HandleBatchFailed(Conn) {
				return
			}
		}

		if app.CommentHandler != nil {
			if app.CommentHandler.HandleSaveCommentData(Conn) {
				return
			}
		}

		if path == "/console" || path == "/console/" {
			consoleHTML, err := os.ReadFile("web/console.html")
			if err != nil {
				utils.Warn("无法读取 web/console.html: %v", err)
				Conn.StopRequest(404, "Console not found", http.Header{})
				return
			}
			headers := http.Header{}
			headers.Set("Content-Type", "text/html; charset=utf-8")
			Conn.StopRequest(200, string(consoleHTML), headers)
			return
		}

		isWeixinResource := strings.Contains(path, "pic_blank.gif") ||
			strings.Contains(path, "we-emoji") ||
			strings.Contains(path, "Expression") ||
			strings.Contains(path, "auth_icon") ||
			strings.Contains(path, "weixin/checkresupdate") ||
			strings.Contains(path, "fed_upload") ||
			strings.HasPrefix(path, "/a/") ||
			strings.HasPrefix(path, "/weixin/")

		if !isWeixinResource && (strings.HasPrefix(path, "/js/") || strings.HasPrefix(path, "/css/") || strings.HasPrefix(path, "/docs/") ||
			strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".md")) {
			filePath := "web" + path
			content, err := os.ReadFile(filePath)
			if err != nil {
				return
			}
			headers := http.Header{}
			if strings.HasSuffix(path, ".js") {
				headers.Set("Content-Type", "application/javascript; charset=utf-8")
			} else if strings.HasSuffix(path, ".css") {
				headers.Set("Content-Type", "text/css; charset=utf-8")
			}
			Conn.StopRequest(200, string(content), headers)
			return
		}

		if strings.HasPrefix(path, "/api/") && app.ConsoleAPIHandler != nil {
			app.handleConsoleAPI(Conn)
			return
		}

		if strings.HasPrefix(path, "/__wx_channels_api/") && Conn.Request.Method == "OPTIONS" {
			headers := http.Header{}
			headers.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			headers.Set("Access-Control-Allow-Headers", "Content-Type, X-Local-Auth")
			if app.Cfg != nil && len(app.Cfg.AllowedOrigins) > 0 {
				origin := Conn.Request.Header.Get("Origin")
				for _, o := range app.Cfg.AllowedOrigins {
					if o == origin {
						headers.Set("Access-Control-Allow-Origin", origin)
						headers.Set("Vary", "Origin")
						break
					}
				}
			}
			Conn.StopRequest(204, "", headers)
			return
		}

		if path == "/__wx_channels_api/save_page_content" {
			var contentData struct {
				URL       string `json:"url"`
				HTML      string `json:"html"`
				Timestamp int64  `json:"timestamp"`
			}
			body, err := io.ReadAll(Conn.Request.Body)
			if err != nil {
				utils.HandleError(err, "读取save_page_content请求体")
				return
			}
			if err := Conn.Request.Body.Close(); err != nil {
				utils.HandleError(err, "关闭请求体")
			}
			err = json.Unmarshal(body, &contentData)
			if err != nil {
				utils.HandleError(err, "解析页面内容数据")
			} else {
				parsedURL, err := url.Parse(contentData.URL)
				if err != nil {
					utils.HandleError(err, "解析页面内容URL")
				} else {
					app.saveDynamicHTML(contentData.HTML, parsedURL, contentData.URL, contentData.Timestamp)
				}
			}
			headers := http.Header{}
			headers.Set("Content-Type", "application/json")
			headers.Set("__debug", "fake_resp")
			Conn.StopRequest(200, "{}", headers)
			return
		}
	}
	if Conn.Type == public.HttpResponseOK {
		if Conn.Response.Body != nil {
			Body, _ := io.ReadAll(Conn.Response.Body)
			_ = Conn.Response.Body.Close()

			if strings.Contains(path, ".js") {
				contentType := strings.ToLower(Conn.Response.Header.Get("content-type"))
				utils.LogInfo("[响应] Path=%s | ContentType=%s", path, contentType)
			}

			if app.ScriptHandler != nil {
				if app.ScriptHandler.HandleHTMLResponse(Conn, host, path, Body) {
					return
				}
			}

			if app.ScriptHandler != nil {
				if app.ScriptHandler.HandleJavaScriptResponse(Conn, host, path, Body) {
					return
				}
			}

			Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
		}
	}
}

// saveDynamicHTML 保存动态页面的完整HTML内容
func (app *App) saveDynamicHTML(htmlContent string, parsedURL *url.URL, fullURL string, timestamp int64) {
	if app.FileManager == nil || app.Cfg == nil {
		utils.Warn("文件管理器或配置未初始化，无法保存页面内容: %s", fullURL)
		return
	}
	if !app.Cfg.SavePageSnapshot {
		return
	}
	if htmlContent == "" || parsedURL == nil {
		return
	}

	if app.Cfg.SaveDelay > 0 {
		time.Sleep(app.Cfg.SaveDelay)
	}

	saveTime := time.Now()
	if timestamp > 0 {
		saveTime = time.Unix(0, timestamp*int64(time.Millisecond))
	}

	downloadsDir, err := utils.ResolveDownloadDir(app.Cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录用于保存页面内容")
		return
	}

	if err := utils.EnsureDir(downloadsDir); err != nil {
		utils.HandleError(err, "创建下载目录用于保存页面内容")
		return
	}

	pagesRoot := filepath.Join(downloadsDir, "page_snapshots")
	if err := utils.EnsureDir(pagesRoot); err != nil {
		utils.HandleError(err, "创建页面保存根目录")
		return
	}

	dateDir := filepath.Join(pagesRoot, saveTime.Format("2006-01-02"))
	if err := utils.EnsureDir(dateDir); err != nil {
		utils.HandleError(err, "创建页面保存日期目录")
		return
	}

	var filenameParts []string
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		segments := strings.Split(parsedURL.Path, "/")
		for _, segment := range segments {
			segment = strings.TrimSpace(segment)
			if segment == "" || segment == "." {
				continue
			}
			filenameParts = append(filenameParts, utils.CleanFilename(segment))
		}
	}

	if parsedURL.RawQuery != "" {
		querySegment := strings.ReplaceAll(parsedURL.RawQuery, "&", "_")
		querySegment = strings.ReplaceAll(querySegment, "=", "-")
		querySegment = utils.CleanFilename(querySegment)
		if querySegment != "" {
			filenameParts = append(filenameParts, querySegment)
		}
	}

	if len(filenameParts) == 0 {
		filenameParts = append(filenameParts, "page")
	}

	baseName := strings.Join(filenameParts, "_")
	fileName := fmt.Sprintf("%s_%s.html", saveTime.Format("150405"), baseName)
	targetPath := utils.GenerateUniqueFilename(dateDir, fileName, 100)

	if err := os.WriteFile(targetPath, []byte(htmlContent), 0644); err != nil {
		utils.HandleError(err, "保存页面HTML内容")
		return
	}

	metaData := map[string]interface{}{
		"url":       fullURL,
		"host":      parsedURL.Host,
		"path":      parsedURL.Path,
		"query":     parsedURL.RawQuery,
		"saved_at":  saveTime.Format(time.RFC3339),
		"timestamp": timestamp,
	}

	metaBytes, err := json.MarshalIndent(metaData, "", "  ")
	if err == nil {
		metaPath := strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".meta.json"
		if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
			utils.HandleError(err, "保存页面元数据")
		}
	}

	utils.LogInfo("[页面快照] 已保存: %s", targetPath)

	utils.PrintSeparator()
	color.Blue("💾 页面快照已保存")
	utils.PrintSeparator()
	utils.PrintLabelValue("📁", "保存路径", targetPath)
	utils.PrintLabelValue("🔗", "页面链接", fullURL)
	utils.PrintSeparator()
	fmt.Println()
	fmt.Println()
}

func (app *App) printDownloadRecordInfo() {
	utils.PrintSeparator()
	color.Blue("📋 下载记录信息")
	utils.PrintSeparator()

	downloadsDir, err := utils.ResolveDownloadDir(app.Cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录")
		return
	}

	recordsPath := filepath.Join(downloadsDir, app.Cfg.RecordsFile)
	utils.PrintLabelValue("📁", "记录文件", recordsPath)
	utils.PrintLabelValue("✏️", "记录格式", "CSV表格格式")
	utils.PrintLabelValue("📊", "记录字段", strings.Join(downloadRecordsHeader, ", "))
	utils.PrintSeparator()
}

func (app *App) printEnvConfig() {
	hasAnyConfig := os.Getenv("WX_CHANNEL_TOKEN") != "" ||
		os.Getenv("WX_CHANNEL_ALLOWED_ORIGINS") != "" ||
		os.Getenv("WX_CHANNEL_LOG_FILE") != "" ||
		os.Getenv("WX_CHANNEL_LOG_MAX_MB") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_PAGE_SNAPSHOT") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_SEARCH_DATA") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_PAGE_JS") != "" ||
		os.Getenv("WX_CHANNEL_SHOW_LOG_BUTTON") != "" ||
		os.Getenv("WX_CHANNEL_UPLOAD_CHUNK_CONCURRENCY") != "" ||
		os.Getenv("WX_CHANNEL_UPLOAD_MERGE_CONCURRENCY") != "" ||
		os.Getenv("WX_CHANNEL_DOWNLOAD_CONCURRENCY") != ""

	if hasAnyConfig {
		utils.PrintSeparator()
		color.Blue("⚙️  环境变量配置信息")
		utils.PrintSeparator()

		if app.Cfg.SecretToken != "" {
			utils.PrintLabelValue("🔐", "安全令牌", "已设置")
		}
		if len(app.Cfg.AllowedOrigins) > 0 {
			utils.PrintLabelValue("🌐", "允许的Origin", strings.Join(app.Cfg.AllowedOrigins, ", "))
		}
		if app.Cfg.LogFile != "" {
			utils.PrintLabelValue("📝", "日志文件", app.Cfg.LogFile)
		}
		if app.Cfg.MaxLogSizeMB > 0 {
			utils.PrintLabelValue("📊", "日志最大大小", fmt.Sprintf("%d MB", app.Cfg.MaxLogSizeMB))
		}
		utils.PrintLabelValue("💾", "保存页面快照", fmt.Sprintf("%v", app.Cfg.SavePageSnapshot))
		utils.PrintLabelValue("🔍", "保存搜索数据", fmt.Sprintf("%v", app.Cfg.SaveSearchData))
		utils.PrintLabelValue("📄", "保存JS文件", fmt.Sprintf("%v", app.Cfg.SavePageJS))
		utils.PrintLabelValue("🖼️", "显示日志按钮", fmt.Sprintf("%v", app.Cfg.ShowLogButton))
		utils.PrintLabelValue("📤", "分片上传并发", app.Cfg.UploadChunkConcurrency)
		utils.PrintLabelValue("🔀", "分片合并并发", app.Cfg.UploadMergeConcurrency)
		utils.PrintLabelValue("📥", "批量下载并发", app.Cfg.DownloadConcurrency)
		utils.PrintSeparator()
	}
}

func (app *App) printTitle() {
	color.Set(color.FgCyan)
	fmt.Println("")
	fmt.Println(" ██╗    ██╗██╗  ██╗     ██████╗██╗  ██╗ █████╗ ███╗   ██╗███╗   ██╗███████╗██╗     ")
	fmt.Println(" ██║    ██║╚██╗██╔╝    ██╔════╝██║  ██║██╔══██╗████╗  ██║████╗  ██║██╔════╝██║     ")
	fmt.Println(" ██║ █╗ ██║ ╚███╔╝     ██║     ███████║███████║██╔██╗ ██║██╔██╗ ██║█████╗  ██║     ")
	fmt.Println(" ██║███╗██║ ██╔██╗     ██║     ██╔══██║██╔══██║██║╚██╗██║██║╚██╗██║██╔══╝  ██║     ")
	fmt.Println(" ╚███╔███╔╝██╔╝ ██╗    ╚██████╗██║  ██║██║  ██║██║ ╚████║██║ ╚████║███████╗███████╗")
	fmt.Println("  ╚══╝╚══╝ ╚═╝  ╚═╝     ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═══╝╚══════╝╚══════╝")
	color.Unset()

	color.Yellow("    微信视频号下载助手 v%s", app.Cfg.Version)
	color.Yellow("    项目地址：https://github.com/nobiyou/wx_channel")
	color.Green("    v%s 更新要点：", app.Cfg.Version)
	color.Green("    • 通用批量下载组件 - 统一UI，减少400+行代码")
	color.Green("    • Home页面分类视频批量下载 - 支持美食、生活等分类")
	color.Green("    • 视频列表优化 - 完整信息显示，分页浏览")
	color.Green("    • 下载功能增强 - 强制重下、取消、实时进度")
	color.Green("    • 搜索页面增强 - 显示直播数据，HTML标签清理")
	color.Green("    • Bug修复 - 下载显示、复选框、标题清理等")
	fmt.Println()
}

// Helpers needed implicitly
type SunnyNetResponseWriter struct {
	conn       *SunnyNet.HttpConn
	headers    http.Header
	statusCode int
	body       bytes.Buffer
}

func NewSunnyNetResponseWriter(conn *SunnyNet.HttpConn) *SunnyNetResponseWriter {
	return &SunnyNetResponseWriter{
		conn:       conn,
		headers:    make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *SunnyNetResponseWriter) Header() http.Header {
	return w.headers
}

func (w *SunnyNetResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *SunnyNetResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *SunnyNetResponseWriter) Flush() {
	w.conn.StopRequest(w.statusCode, w.body.String(), w.headers)
}

func (app *App) handleConsoleAPI(Conn *SunnyNet.HttpConn) {
	w := NewSunnyNetResponseWriter(Conn)
	app.ConsoleAPIHandler.HandleAPIRequest(w, Conn.Request)
	w.Flush()
}

func (app *App) startWebSocketServer(wsPort int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		handlers.ServeWs(w, r)
	})

	wsHandler := websocket.NewHandler(app.WSHub)
	mux.HandleFunc("/ws/api", wsHandler.ServeHTTP)

	mux.HandleFunc("/ws/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		hub := handlers.GetWebSocketHub()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"clients": hub.ClientCount(),
		})
	})

	if app.SearchService != nil {
		mux.HandleFunc("/api/channels/contact/search", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			app.SearchService.SearchContact(w, r)
		})

		mux.HandleFunc("/api/channels/contact/feed/list", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			app.SearchService.GetFeedList(w, r)
		})

		mux.HandleFunc("/api/channels/feed/profile", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			app.SearchService.GetFeedProfile(w, r)
		})

		mux.HandleFunc("/api/channels/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			app.SearchService.GetStatus(w, r)
		})
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", wsPort),
		Handler: mux,
	}

	utils.Info("🔌 WebSocket服务已启动，端口: %d", wsPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		utils.Warn("WebSocket服务启动失败: %v", err)
	}
}
