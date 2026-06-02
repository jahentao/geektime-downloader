package batch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/briandowns/spinner"

	"github.com/nicoxiang/geektime-downloader/internal/config"
	"github.com/nicoxiang/geektime-downloader/internal/course"
	"github.com/nicoxiang/geektime-downloader/internal/geektime"
	"github.com/nicoxiang/geektime-downloader/internal/pkg/filenamify"
	"github.com/nicoxiang/geektime-downloader/internal/pkg/logger"
	"github.com/nicoxiang/geektime-downloader/internal/ui"
)

// CourseItem represents a course to download
type CourseItem struct {
	ID         int
	ColumnType int
}

// BatchDownloader downloads all purchased and VIP courses, and uploads them to Baidu Pan
func BatchDownloader(
	ctx context.Context,
	cfg *config.AppConfig,
	client *geektime.Client,
) error {
	fmt.Println("正在获取已订阅课程列表（含VIP免费课程）...")

	// 1. 获取专栏 (type=1)
	columnsResp, err := client.NewAllColumns(1)
	if err != nil {
		return fmt.Errorf("获取专栏列表失败: %w", err)
	}

	// 2. 获取视频课 (type=3)
	videosResp, err := client.NewAllColumns(3)
	if err != nil {
		return fmt.Errorf("获取视频课列表失败: %w", err)
	}

	var courses []CourseItem
	var columnCount, videoCount int

	// 专栏优先
	for _, item := range columnsResp.Data.List {
		if item.HadSub {
			courses = append(courses, CourseItem{ID: item.ID, ColumnType: item.ColumnType})
			columnCount++
		}
	}

	// 视频课次之
	for _, item := range videosResp.Data.List {
		if item.HadSub {
			courses = append(courses, CourseItem{ID: item.ID, ColumnType: item.ColumnType})
			videoCount++
		}
	}

	fmt.Printf("共获取到 %d 门已订阅课程（专栏: %d, 视频课: %d）\n\n",
		len(courses), columnCount, videoCount)

	if len(courses) == 0 {
		fmt.Println("没有可下载的课程")
		return nil
	}

	sp := spinner.New(spinner.CharSets[4], 100*time.Millisecond)
	cd := course.NewCourseDownloader(ctx, cfg, client, sp)

	// 读取百度网盘配置
	baiduPanDir := os.Getenv("BAIDU_PAN_DIR")
	if baiduPanDir == "" {
		baiduPanDir = "/apps/geektime-courses"
	}
	cleanupAfterUpload := os.Getenv("CLEANUP_AFTER_UPLOAD") == "1"
	uploadEnabled := os.Getenv("DISABLE_UPLOAD") != "1"

	// 确保网盘目录存在
	if uploadEnabled {
		_ = exec.Command("BaiduPCS-Go", "mkdir", baiduPanDir).Run()
	}

	// 上传并发控制（最多3个同时上传）
	uploadSem := make(chan struct{}, 3)
	var uploadWg sync.WaitGroup

	// 统计
	var downloadOK, downloadFail, uploadOK, uploadFail int
	var statMu sync.Mutex

	for idx, item := range courses {
		cid := item.ID
		fmt.Printf("[%d/%d] 正在处理课程 ID: %d ...\n", idx+1, len(courses), cid)

		// 获取课程信息（带重试）
		var courseInfo geektime.Course
		var courseErr error
		for attempt := 1; attempt <= 3; attempt++ {
			courseInfo, courseErr = client.CourseInfo(cid)
			if courseErr == nil {
				break
			}
			logger.Errorf(courseErr, "获取课程信息失败, column_id: %d, 重试 %d/3", cid, attempt)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
		}
		if courseErr != nil {
			logger.Errorf(courseErr, "获取课程信息失败, column_id: %d", cid)
			fmt.Fprintf(os.Stderr, "获取课程信息失败 (column_id: %d)，跳过\n", cid)
			statMu.Lock()
			downloadFail++
			statMu.Unlock()
			continue
		}

		if !courseInfo.Access {
			fmt.Fprintf(os.Stderr, "无权限访问课程: %s，跳过\n", courseInfo.Title)
			statMu.Lock()
			downloadFail++
			statMu.Unlock()
			continue
		}

		fmt.Printf("  课程: 《%s》(%s)\n", courseInfo.Title, courseInfo.Type)

		productType := determineProductType(courseInfo)

		// 下载课程（带重试）
		var downloadErr error
		for attempt := 1; attempt <= 3; attempt++ {
			downloadErr = cd.DownloadAll(courseInfo, productType)
			if downloadErr == nil {
				break
			}
			logger.Errorf(downloadErr, "下载课程失败: %s, 重试 %d/3", courseInfo.Title, attempt)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 3 * time.Second)
			}
		}

		if downloadErr != nil {
			logger.Errorf(downloadErr, "下载课程失败: %s", courseInfo.Title)
			fmt.Fprintf(os.Stderr, "下载课程失败: %s，错误: %v\n", courseInfo.Title, downloadErr)
			statMu.Lock()
			downloadFail++
			statMu.Unlock()
			continue
		}

		fmt.Printf("  ✓ 《%s》 下载完成\n", courseInfo.Title)
		statMu.Lock()
		downloadOK++
		statMu.Unlock()

		// 并行上传
		if uploadEnabled {
			courseDir := filepath.Join(cfg.DownloadFolder, filenamify.Filenamify(courseInfo.Title))
			uploadWg.Add(1)
			uploadSem <- struct{}{}
			go func(dir, name string, id int) {
				defer uploadWg.Done()
				defer func() { <-uploadSem }()

				fmt.Printf("  ⬆ 开始上传: %s ...\n", name)
				if err := uploadCourse(dir, name, baiduPanDir, cleanupAfterUpload); err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ %s 上传失败: %v\n", name, err)
					statMu.Lock()
					uploadFail++
					statMu.Unlock()
				} else {
					fmt.Printf("  ✓ %s 上传成功\n", name)
					statMu.Lock()
					uploadOK++
					statMu.Unlock()
				}
			}(courseDir, courseInfo.Title, cid)
		}
	}

	// 等待所有上传完成
	if uploadEnabled {
		fmt.Println("\n等待所有上传任务完成...")
		uploadWg.Wait()
	}

	fmt.Println("\n========================================")
	fmt.Println("✅ 所有任务完成")
	fmt.Printf("下载: 成功 %d, 失败 %d\n", downloadOK, downloadFail)
	if uploadEnabled {
		fmt.Printf("上传: 成功 %d, 失败 %d\n", uploadOK, uploadFail)
	}
	fmt.Println("========================================")
	return nil
}

func uploadCourse(courseDir, courseName, baiduPanDir string, cleanup bool) error {
	if _, err := os.Stat(courseDir); os.IsNotExist(err) {
		return fmt.Errorf("本地目录不存在: %s", courseDir)
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.Command("BaiduPCS-Go", "upload",
			"--policy", "rsync",
			"--retry", "3",
			courseDir, baiduPanDir,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		lastErr = cmd.Run()
		if lastErr == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
	}

	if lastErr != nil {
		return lastErr
	}

	if cleanup {
		_ = os.RemoveAll(courseDir)
		fmt.Printf("  🗑 已删除本地文件: %s\n", courseName)
	}
	return nil
}

func determineProductType(c geektime.Course) ui.ProductTypeSelectOption {
	pt := ui.ProductTypeSelectOption{
		Index:              0,
		Text:               "普通课程",
		SourceType:         1,
		AcceptProductTypes: []string{"c1", "c3"},
		NeedSelectArticle:  true,
		IsEnterpriseMode:   false,
	}

	switch c.Type {
	case "c1":
		pt.Text = "专栏"
	case "c3":
		pt.Text = "视频课"
	case "d":
		pt.Index = 1
		pt.Text = "每日一课"
		pt.SourceType = 2
		pt.AcceptProductTypes = []string{"d"}
		pt.NeedSelectArticle = false
	case "q":
		pt.Index = 3
		pt.Text = "大厂案例"
		pt.SourceType = 4
		pt.AcceptProductTypes = []string{"q"}
		pt.NeedSelectArticle = false
	default:
		pt.Text = "其他"
		pt.AcceptProductTypes = []string{c.Type}
	}

	return pt
}
