package token

import (
	"io/ioutil"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileToken is a TokenProvider for a token which is backed by a file.
// This will lookup the value from the file, and will watch the file for
// changes, and re-read when required.
//
// This is typically used for in-cluster service account tokens, which
// Kubernetes mounts into the pod at
// /var/run/secrets/kubernets.io/serviceaccount/token, and will change
// this file if and when the token expires and is reissued.
type FileToken struct {
	mutex sync.RWMutex
	token string
}

func NewFileToken(filename string) (*FileToken, error) {

	value, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fileToken := FileToken{
		token: string(value),
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					value, err := ioutil.ReadFile(filename)
					if err == nil {
						fileToken.mutex.Lock()
						fileToken.token = string(value)
						fileToken.mutex.Unlock()
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(filename)
	if err != nil {
		return nil, err
	}

	return &fileToken, nil
}

func (t *FileToken) Token() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.token
}
