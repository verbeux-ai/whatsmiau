package whatsmiau

import (
	"sync"
	"testing"
	"time"
)

func TestRunLoadMiauTasksLimitsConcurrencyAndWaits(t *testing.T) {
	total := loadMiauConnectConcurrency + 5
	items := make([]int, total)
	started := make(chan struct{}, total)
	release := make(chan struct{})
	done := make(chan struct{})

	var stateMu sync.Mutex
	active := 0
	maxActive := 0
	completed := 0

	go func() {
		runLoadMiauTasks(items, func(int) {
			stateMu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			stateMu.Unlock()

			started <- struct{}{}
			<-release

			stateMu.Lock()
			active--
			completed++
			stateMu.Unlock()
		})
		close(done)
	}()

	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	for range loadMiauConnectConcurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initial tasks")
		}
	}

	select {
	case <-started:
		t.Fatalf("more than %d tasks started concurrently", loadMiauConnectConcurrency)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	released = true

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for all tasks to complete")
	}

	stateMu.Lock()
	defer stateMu.Unlock()
	if maxActive != loadMiauConnectConcurrency {
		t.Fatalf("expected max concurrency %d, got %d", loadMiauConnectConcurrency, maxActive)
	}
	if completed != total {
		t.Fatalf("expected %d completed tasks, got %d", total, completed)
	}
}
