package filewatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	path   string
	done   chan struct{}
	notify func(path string, err error)
}

func New(path string, notify func(string, error)) (fw FileWatcher, err error) {
	fw = FileWatcher{
		path: path,

		notify: notify,
		done:   make(chan struct{}),
	}
	go fw.watchLoop()
	return
}

func (fw FileWatcher) watchLoop() {
	for {
		select {
		case <-fw.done:
			return

		default:
		}

		if err := fw.watch(); err != nil {
			fw.notify("", fmt.Errorf("cert watcher stopped due to error: %w", err))
			<-time.After(time.Minute)
		}
	}
}

func (fw FileWatcher) watch() (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fail to instantiate fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	if err = watcher.Add(fw.path); err != nil {
		return fmt.Errorf("fail to watch %s: %w", fw.path, err)
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
				return fmt.Errorf("fail to re-add watched file: %w", err)
			}

		default:
			continue
		}

		fw.notify(event.Name, nil)
	}
}

func (fw FileWatcher) Close(ctx context.Context) (err error) {
	close(fw.done)
	return
}
