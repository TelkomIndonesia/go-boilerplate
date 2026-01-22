package filewatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatch struct {
	path   string
	notify func(path string, err error)

	done     chan struct{}
	doneOnce sync.Once
}

func New(path string, notify func(string, error)) (fw *FileWatch, err error) {
	fw = &FileWatch{
		path: path,

		notify: notify,
		done:   make(chan struct{}),
	}
	go fw.watchLoop()
	return
}

func (fw *FileWatch) watchLoop() {
	var delay = time.Minute
	var last = time.Now().Add(-time.Minute)
	for {
		d := delay - time.Since(last)
		select {
		case <-fw.done:
			return

		case <-time.After(d):
		}

		last = time.Now()
		if err := fw.watch(); err != nil {
			fw.notify("", fmt.Errorf("cert watcher stopped due to error: %w", err))
		}
	}
}

func (fw *FileWatch) watch() (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to instantiate fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	if err = watcher.Add(fw.path); err != nil {
		return fmt.Errorf("failed to watch %s: %w", fw.path, err)
	}

	for {
		var event fsnotify.Event

		select {
		case <-fw.done:
			return

		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}

			fw.notify("", fmt.Errorf("error event received: %w", err))
			continue

		case e, ok := <-watcher.Events:
			if !ok {
				return
			}

			event = e
		}

		switch {
		case event.Has(fsnotify.Write):
		case event.Has(fsnotify.Remove) || event.Has(fsnotify.Chmod):
			watcher.Remove(fw.path)
			if err := watcher.Add(fw.path); err != nil {
				return fmt.Errorf("failed to re-add watched file: %w", err)
			}

		default:
			continue
		}

		fw.notify(event.Name, nil)
	}
}

func (fw *FileWatch) Close(ctx context.Context) (err error) {
	fw.doneOnce.Do(func() {
		close(fw.done)
	})
	return
}
