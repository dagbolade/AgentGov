package policy

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

type ChangeHandler func(path string)

type FileWatcher struct {
	watcher *fsnotify.Watcher
	dir     string
	handler ChangeHandler
	done    chan struct{}
}

func NewFileWatcher(dir string, handler ChangeHandler) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch directory: %w", err)
	}

	fw := &FileWatcher{
		watcher: watcher,
		dir:     dir,
		handler: handler,
		done:    make(chan struct{}),
	}

	go fw.watch()

	return fw, nil
}

func (fw *FileWatcher) Close() error {
	close(fw.done)
	return fw.watcher.Close()
}

func (fw *FileWatcher) watch() {
	debounce := time.NewTimer(0)
	<-debounce.C // Drain initial timer

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if fw.shouldHandle(event) {
				// Debounce rapid changes
				debounce.Reset(500 * time.Millisecond)
				go fw.waitAndHandle(debounce, event.Name)
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("watcher error")

		case <-fw.done:
			return
		}
	}
}

func (fw *FileWatcher) shouldHandle(event fsnotify.Event) bool {
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return false
	}

	ext := filepath.Ext(event.Name)
	return ext == ".wasm"
}

func (fw *FileWatcher) waitAndHandle(timer *time.Timer, path string) {
	<-timer.C
	fw.handler(path)
}