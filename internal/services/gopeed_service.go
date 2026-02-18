package services

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"time"
	"wx_channel/internal/utils"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	_ "github.com/GopeedLab/gopeed/pkg/protocol/http" // Register HTTP protocol
	httpProtocol "github.com/GopeedLab/gopeed/pkg/protocol/http"
)

// GopeedService wraps the Gopeed downloader engine
type GopeedService struct {
	Downloader *download.Downloader
	mu         sync.RWMutex
	tasks      map[string]string // Maps internal ID to Gopeed Task ID
}

// NewGopeedService creates a new GopeedService
// Note: We bypass store for now due to dependency issues or signature changes
func NewGopeedService(storageDir string) *GopeedService {
	// Create downloader config
	dlCfg := &download.DownloaderConfig{
		// Default config is acceptable
	}

	// Create a downloader instance
	d := download.NewDownloader(dlCfg)

	// Try to setup
	if err := d.Setup(); err != nil {
		utils.Warn("Gopeed Setup failed: %v", err)
	}

	return &GopeedService{
		Downloader: d,
		tasks:      make(map[string]string),
	}
}

// CreateTask creates a download task
func (s *GopeedService) CreateTask(url string, opt *base.Options) (string, error) {
	if s.Downloader == nil {
		return "", fmt.Errorf("downloader not initialized")
	}
	req := &base.Request{URL: url}
	return s.Downloader.CreateDirect(req, opt)
}

// DownloadSync downloads a file synchronously (blocking until done)
// Used by BatchHandler to replace existing downloadVideoOnce logic
func (s *GopeedService) DownloadSync(ctx context.Context, url string, path string, connections int, onProgress func(progress float64, downloaded int64, total int64)) error {
	if s.Downloader == nil {
		return fmt.Errorf("downloader not initialized")
	}

	// Configure options
	dir := filepath.Dir(path)
	name := filepath.Base(path)

	// 默认8个连接
	if connections <= 0 {
		connections = 8
	}

	opts := &base.Options{
		Path:  dir,
		Name:  name,
		Extra: &httpProtocol.OptsExtra{
			Connections: connections, // 单文件多线程下载
		},
	}

	// Create task using CreateDirect
	req := &base.Request{URL: url}
	id, err := s.Downloader.CreateDirect(req, opts)
	if err != nil {
		return fmt.Errorf("failed to create task: %v", err)
	}

	// Poll status
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Cancel task
			s.Downloader.Pause(&download.TaskFilter{IDs: []string{id}})
			// Or remove
			return ctx.Err()
		case <-ticker.C:
			task := s.Downloader.GetTask(id)
			if task == nil {
				return fmt.Errorf("task not found: %s", id)
			}

			// Report progress
			if onProgress != nil {
				// Gopeed task has TotalSize and CompletedLength?
				// Verify properties. Inspecting base.Task might be needed.
				// Assuming standard fields based on similar libraries:
				// task.Meta.FileSize -> Total
				// But let's look safely.
				// Does task have Progress?
				// task.Progress is likely 0.0-1.0 or 0-100?
				// Let's assume task.Status is updated, maybe bytes happen too.
				// Based on typical Gopeed usage:
				// task.Res.Size (Total), task.Res.Downloaded (Current) -> not sure if exposed in Task struct directly or via Res.
				// Let's check imports: "github.com/GopeedLab/gopeed/pkg/base"
				// I'll assume reasonable defaults and if it fails compilation I'll fix.
				// Better strategy: try to dump task structure via printf debugging? No, strictly code.
				// I'll assume task.Total is total bytes and task.Completed is downloaded bytes if available?
				// Let's guess task.Progress (float 0-1) is available.
				// Wait, I can't guess. I previously read gopeed_service.go.
				// It used `task.Status`.
				// I'll try to find where `base.Task` is defined or use `task.Progress` which is common.
				// Actually, let's look at `BatchTask` struct in batch.go: it has `Progress`, `DownloadedMB`, `TotalMB`.
				// I want to fill those.
				// Let's write code that assumes task has typical methods or fields.
				// For now I will just pass 0,0,0 if I'm unsure, but that defeats the purpose.
				// Let's check `gopeed` source if it was vendored? No, likely in module cache.
				// I will try to use `task.TotalSize` and `task.DownloadedSize` based on common patterns.
				// If unsafe, I'll pass 0.
				// Actually, I'll inspect `gopeed_service.go` again to see if I missed any logic.
				// It imports `github.com/GopeedLab/gopeed/pkg/base`.
			}

			// Report progress
			if onProgress != nil && task != nil {
				var downloaded, total int64
				var progress float64

				if task.Progress != nil {
					downloaded = task.Progress.Downloaded
				}

				// 使用反射获取 TotalSize (因为 Meta 字段类型是 internal 的，外部无法直接访问)
				// task.Meta -> *fetcher.FetcherMeta
				// FetcherMeta.Res -> *base.Resource
				// Resource.Size -> int64
				func() {
					defer func() {
						if r := recover(); r != nil {
							// 忽略反射 panic，防止 crash
							utils.Warn("反射获取文件大小失败: %v", r)
						}
					}()

					// get *Task value
					v := reflect.ValueOf(task).Elem()

					// get Meta field
					metaField := v.FieldByName("Meta")
					if metaField.IsValid() && !metaField.IsNil() {
						// get Res field from FetcherMeta
						// FetcherMeta struct definition: type FetcherMeta struct { ... Res *base.Resource ... }
						// We need to dereference the pointer first
						resField := metaField.Elem().FieldByName("Res")
						if resField.IsValid() && !resField.IsNil() {
							// get Size field from Resource
							sizeField := resField.Elem().FieldByName("Size")
							if sizeField.IsValid() {
								total = sizeField.Int()
							}
						}
					}
				}()

				// 最后一次尝试：Task 通常有 TotalSize?
				// total = task.TotalSize

				if total > 0 {
					progress = float64(downloaded) / float64(total)
				}

				// 调用进度回调
				onProgress(progress, downloaded, total)
			}

			// Check status
			switch task.Status {
			case base.DownloadStatusDone:
				return nil
			case base.DownloadStatusError:
				return fmt.Errorf("download task failed")
			case base.DownloadStatusRunning, base.DownloadStatusReady:
				// Continue waiting
				continue
			default:
				// Handle other statuses (Paused, etc)
				// Continue waiting
			}
		}
	}
}
