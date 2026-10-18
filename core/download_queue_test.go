package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
)

func queueInputs(n int) []DownloadTaskInput {
	inputs := make([]DownloadTaskInput, n)
	for i := range inputs {
		inputs[i] = DownloadTaskInput{Song: model.Song{ID: fmt.Sprint(i), Source: "kuwo", Name: fmt.Sprintf("Song %d", i), Artist: "Artist"}, Directory: "/downloads", Embed: true}
	}
	return inputs
}
func testDownloadQueue(t *testing.T) *DownloadQueue {
	t.Helper()
	newDownloadDedupTestDB(t)
	q, err := OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func TestDownloadQueueDurableBatchAndActiveDedup(t *testing.T) {
	q := testDownloadQueue(t)
	if n, err := q.Enqueue("batch-1", queueInputs(550)); err != nil || n != 550 {
		t.Fatalf("enqueue %d: %v", n, err)
	}
	if n, err := q.Enqueue("batch-1", queueInputs(550)); err != nil || n != 0 {
		t.Fatalf("idempotent enqueue %d: %v", n, err)
	}
	if n, err := q.Enqueue("batch-2", queueInputs(550)); err != nil || n != 0 {
		t.Fatalf("active dedup %d: %v", n, err)
	}
	if err := CloseConfigDB(); err != nil {
		t.Fatal(err)
	}
	q, err := OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	tasks, total, counts, err := q.List(11, 50, "")
	if err != nil || total != 550 || len(tasks) != 50 || counts[TaskWaiting] != 550 {
		t.Fatalf("restored page %d/%d %v %v", len(tasks), total, counts, err)
	}
	bad := queueInputs(2)
	bad[1].Song.ID = ""
	if n, err := q.Enqueue("bad", bad); err == nil || n != 0 {
		t.Fatalf("invalid batch should roll back: %d %v", n, err)
	}
}
func TestDownloadQueueRecoveryContinuesAndKeepsTerminalStates(t *testing.T) {
	q := testDownloadQueue(t)
	if _, err := q.Enqueue("restore", queueInputs(4)); err != nil {
		t.Fatal(err)
	}
	first, err := q.claim()
	if err != nil {
		t.Fatal(err)
	}
	if err := q.finish(first.ID, DownloadTaskResult{Status: DownloadStatusSuccess}, nil); err != nil {
		t.Fatal(err)
	}
	interrupted, err := q.claim()
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Name != "Song 1" {
		t.Fatal(interrupted)
	}
	if err := CloseConfigDB(); err != nil {
		t.Fatal(err)
	}
	q, err = OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	if err = q.recoverInterrupted(); err != nil {
		t.Fatal(err)
	}
	var order []string
	for i := 0; i < 3; i++ {
		task, err := q.claim()
		if err != nil {
			t.Fatal(err)
		}
		order = append(order, task.Name)
		result := DownloadTaskResult{Status: DownloadStatusSuccess}
		var processErr error
		if i == 1 {
			processErr = errors.New("network unavailable")
		}
		if err = q.finish(task.ID, result, processErr); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprint(order) != "[Song 1 Song 2 Song 3]" {
		t.Fatal(order)
	}
	_, total, counts, err := q.List(1, 50, "")
	if err != nil || total != 4 || counts[DownloadStatusSuccess] != 3 || counts[DownloadStatusFailed] != 1 {
		t.Fatalf("%d %v %v", total, counts, err)
	}
}
func TestDownloadQueueAppendCancelRetryAndWorker(t *testing.T) {
	q := testDownloadQueue(t)
	_, err := q.Enqueue("initial", queueInputs(2))
	if err != nil {
		t.Fatal(err)
	}
	tasks, _, _, _ := q.List(1, 50, "")
	if err = q.Cancel(tasks[1].ID); err != nil {
		t.Fatal(err)
	}
	if err = q.Retry(tasks[1].ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var seen []string
	err = q.Start(ctx, func(input DownloadTaskInput) (DownloadTaskResult, error) {
		mu.Lock()
		seen = append(seen, input.Song.Name)
		mu.Unlock()
		return DownloadTaskResult{Status: DownloadStatusSuccess}, nil
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	more := queueInputs(1)
	more[0].Song.ID = "new"
	more[0].Song.Name = "Appended"
	if n, err := q.Enqueue("appended", more); err != nil || n != 1 {
		cancel()
		t.Fatalf("%d %v", n, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, _, counts, err := q.List(1, 50, "")
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if counts[DownloadStatusSuccess] == 3 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("worker did not finish", counts)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-q.done
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(seen) != "[Song 0 Song 1 Appended]" {
		t.Fatal(seen)
	}
}
func TestDownloadQueuePanicIsAFailedTask(t *testing.T) {
	_, err := processDownloadTask(func(DownloadTaskInput) (DownloadTaskResult, error) { panic("provider crashed") }, DownloadTaskInput{})
	if err == nil {
		t.Fatal("processor panic must be captured")
	}
}
func TestDownloadQueueConcurrentBatches(t *testing.T) {
	q := testDownloadQueue(t)
	var wg sync.WaitGroup
	failures := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, err := q.Enqueue(fmt.Sprint(i), queueInputs(20)); failures <- err }(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 20 {
		t.Fatalf("%d %v", total, err)
	}
}

func TestDownloadQueueRemoveAndClearKeepInFlightIdentityUntilFinished(t *testing.T) {
	q := testDownloadQueue(t)
	if _, err := q.Enqueue("remove", queueInputs(3)); err != nil {
		t.Fatal(err)
	}
	active, err := q.claim()
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Remove(active.ID); err != nil {
		t.Fatal(err)
	}
	tasks, total, counts, err := q.List(1, 50, "")
	if err != nil || total != 2 || counts[TaskDownloading] != 0 {
		t.Fatal(total, counts, err)
	}
	if added, err := q.Enqueue("duplicate", queueInputs(1)); err != nil || added != 0 {
		t.Fatal("in-flight dedup lost", added, err)
	}
	if err := q.Remove(tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := q.Clear(); err != nil {
		t.Fatal(err)
	}
	if next, err := q.claim(); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cleared task was claimed", next, err)
	}
	if err := q.finish(active.ID, DownloadTaskResult{Status: DownloadStatusSuccess, Path: "/keep/audio.mp3"}, nil); err != nil {
		t.Fatal(err)
	}
	_, total, counts, err = q.List(1, 50, "")
	if err != nil || total != 0 || len(counts) != 0 {
		t.Fatal("task reappeared", total, counts, err)
	}
	if added, err := q.Enqueue("after-finish", queueInputs(1)); err != nil || added != 1 {
		t.Fatal(added, err)
	}
}

func TestDownloadQueueClearDoesNotRestoreRemovedTaskOnRestart(t *testing.T) {
	q := testDownloadQueue(t)
	if _, err := q.Enqueue("restart-clear", queueInputs(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := q.claim(); err != nil {
		t.Fatal(err)
	}
	if err := q.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := q.recoverInterrupted(); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := q.db.Model(&DownloadTask{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
