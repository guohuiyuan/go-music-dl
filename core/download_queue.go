package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TaskWaiting     = "waiting"
	TaskDownloading = "downloading"
	TaskCanceled    = "canceled"
)

// DownloadTaskInput stores metadata, not expiring stream URLs. Settings are captured
// when enqueued so a later directory change cannot redirect an existing batch.
type DownloadTaskInput struct {
	Song             model.Song
	Directory        string
	FilenameTemplate string
	Embed            bool
}

type DownloadTask struct {
	ID         uint   `gorm:"primaryKey"`
	RequestKey string `gorm:"uniqueIndex" json:"-"`
	TaskKey    string `gorm:"index" json:"-"`
	Payload    string `gorm:"type:text;not null" json:"-"`
	Name       string
	Artist     string
	Source     string
	Directory  string
	Status     string `gorm:"index"`
	Attempts   int
	Error      string
	Warning    string
	Path       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type DownloadTaskResult struct{ Status, Path, Warning string }
type DownloadTaskProcessor func(DownloadTaskInput) (DownloadTaskResult, error)

type DownloadQueue struct {
	db       *gorm.DB
	start    sync.Once
	startErr error
	mu       sync.Mutex
	done     chan struct{}
}

func OpenDownloadQueue() (*DownloadQueue, error) {
	if err := ensureConfigDB(); err != nil {
		return nil, err
	}
	if err := configDB.AutoMigrate(&DownloadTask{}); err != nil {
		return nil, err
	}
	if err := configDB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS download_tasks_active_key ON download_tasks(task_key) WHERE status IN ('waiting', 'downloading')").Error; err != nil {
		return nil, err
	}
	return &DownloadQueue{db: configDB, done: make(chan struct{})}, nil
}

// Enqueue commits the entire batch before acknowledging it to the client.
func (q *DownloadQueue) Enqueue(requestID string, inputs []DownloadTaskInput) (added int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if strings.TrimSpace(requestID) == "" || len(requestID) > 128 || len(inputs) == 0 || len(inputs) > 20000 {
		return 0, errors.New("invalid download batch")
	}
	err = q.db.Transaction(func(tx *gorm.DB) error {
		for i, input := range inputs {
			if strings.TrimSpace(input.Song.ID) == "" || strings.TrimSpace(input.Song.Source) == "" {
				return errors.New("missing song id or source")
			}
			payload, err := json.Marshal(input)
			if err != nil {
				return err
			}
			identity, _ := json.Marshal([]string{input.Song.Source, input.Song.ID, input.Directory})
			task := DownloadTask{RequestKey: fmt.Sprintf("%s:%d", requestID, i), TaskKey: fmt.Sprintf("%x", sha256.Sum256(identity)), Payload: string(payload), Name: input.Song.Name, Artist: input.Song.Artist, Source: input.Song.Source, Directory: input.Directory, Status: TaskWaiting}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&task)
			if result.Error != nil {
				return result.Error
			}
			added += int(result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		added = 0
	}
	return
}

func (q *DownloadQueue) List(page, size int, status string) ([]DownloadTask, int64, map[string]int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	counts := map[string]int64{}
	var groups []struct {
		Status string
		Count  int64
	}
	if err := q.db.Model(&DownloadTask{}).Select("status, count(*) as count").Group("status").Scan(&groups).Error; err != nil {
		return nil, 0, nil, err
	}
	for _, g := range groups {
		counts[g.Status] = g.Count
	}
	query := q.db.Model(&DownloadTask{})
	if status == "active" {
		query = query.Where("status IN ?", []string{TaskWaiting, TaskDownloading})
	} else if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, nil, err
	}
	tasks := []DownloadTask{}
	err := query.Order("CASE status WHEN 'downloading' THEN 0 WHEN 'waiting' THEN 1 ELSE 2 END, CASE WHEN status IN ('waiting', 'downloading') THEN id END ASC, id DESC").Offset((page - 1) * size).Limit(size).Find(&tasks).Error
	return tasks, total, counts, err
}

func (q *DownloadQueue) Retry(id uint) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := q.db.Model(&DownloadTask{}).Where("id = ? AND status IN ?", id, []string{DownloadStatusFailed, TaskCanceled}).Updates(map[string]interface{}{"status": TaskWaiting, "error": "", "warning": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("task cannot be retried")
	}
	return nil
}
func (q *DownloadQueue) Cancel(id uint) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := q.db.Model(&DownloadTask{}).Where("id = ? AND status = ?", id, TaskWaiting).Update("status", TaskCanceled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("only waiting tasks can be canceled")
	}
	return nil
}

func (q *DownloadQueue) recoverInterrupted() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.db.Model(&DownloadTask{}).Where("status = ?", TaskDownloading).Updates(map[string]interface{}{"status": TaskWaiting, "error": ""}).Error
}
func (q *DownloadQueue) claim() (*DownloadTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var task DownloadTask
	err := q.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("status = ?", TaskWaiting).Order("id ASC").Limit(1).Find(&task)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		result := tx.Model(&DownloadTask{}).Where("id = ? AND status = ?", task.ID, TaskWaiting).Updates(map[string]interface{}{"status": TaskDownloading, "attempts": gorm.Expr("attempts + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	return &task, err
}
func (q *DownloadQueue) finish(id uint, result DownloadTaskResult, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	status := result.Status
	message := ""
	if err != nil {
		status = DownloadStatusFailed
		message = err.Error()
	}
	if status != DownloadStatusSuccess && status != DownloadStatusSkipped && status != DownloadStatusFailed {
		status = DownloadStatusFailed
		message = "invalid download result"
	}
	return q.db.Model(&DownloadTask{}).Where("id = ? AND status = ?", id, TaskDownloading).Updates(map[string]interface{}{"status": status, "error": message, "warning": result.Warning, "path": result.Path}).Error
}
func processDownloadTask(process DownloadTaskProcessor, input DownloadTaskInput) (result DownloadTaskResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("download panic: %v", r)
		}
	}()
	return process(input)
}

// Start is called once after the server has bound its port. A single worker keeps
// FIFO ordering and avoids concurrent writes to the same song's output file.
func (q *DownloadQueue) Start(ctx context.Context, process DownloadTaskProcessor) error {
	q.start.Do(func() {
		if q.startErr = q.recoverInterrupted(); q.startErr != nil {
			return
		}
		go func() {
			defer close(q.done)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				if ctx.Err() != nil {
					return
				}
				task, err := q.claim()
				if err == nil {
					var input DownloadTaskInput
					err = json.Unmarshal([]byte(task.Payload), &input)
					var result DownloadTaskResult
					if err == nil {
						result, err = processDownloadTask(process, input)
					}
					for {
						if saveErr := q.finish(task.ID, result, err); saveErr == nil {
							break
						} else {
							log.Printf("persist download task %d: %v", task.ID, saveErr)
						}
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
						}
					}
					continue
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					log.Printf("claim download task: %v", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
	return q.startErr
}
