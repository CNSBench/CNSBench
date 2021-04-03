/* tail.go
Functions for tail-like reading of log files.
Accounts for log rotation and file truncation.
*/

package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	fsnotify "gopkg.in/fsnotify/fsnotify.v1"
)

type logreader struct {
	fd *os.File
	reader *bufio.Reader
	curReadPos int64
	callback func(string) error
}

func (l *logreader) readLines() (err error) {
	var line string
	for {
		line, err = l.reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSuffix(line, "\n")
		l.curReadPos += int64(len(line))
		l.callback(line)
	}
}

func (l *logreader) handleFsnotifyWrite() error {
	// Try reading lines first. If non-EOF error encountered, return
	if err := l.readLines(); err != io.EOF {
		return err
	}
	// Once EOF encountered, check if file was truncated
	if fi, err := l.fd.Stat(); err != nil {
		return err
	} else if fi.Size() < l.curReadPos {
		// If file was truncated, reset file descriptor offset
		l.curReadPos = 0
		l.fd.Seek(0, io.SeekStart)
		return l.readLines()
	}else {
		// If file was not truncated, just return EOF
		return io.EOF
	}
}

func tail(watcher *fsnotify.Watcher, filename string, errchan chan error, callback func(string) error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	// Initialize logreader (file descriptor, reader, curReadPos)
	l := &logreader{}
	if fd, err := os.Open(filename); err != nil {
		errchan <- err
		return
	} else {
		l.fd = fd
		defer func(l *logreader) {
			l.fd.Close()
		}(l)
	}
	l.reader = bufio.NewReader(l.fd)
	l.curReadPos = 0
	l.callback = callback

	// Read existing lines in file
	if err := l.readLines(); err != io.EOF {
		errchan <- err
		return
	}

	// After existing lines are read, start event handling
	for {
		select {
		case <-signals:
			errchan <- nil
			return
		case event, ok := <- watcher.Events:
			if !ok {
				errchan <- errors.New("watcher.Events error")
				return
			}
			if (event.Op & fsnotify.Create == fsnotify.Create) && (event.Name == filename) {
				// If filename is created, reopen filename
				if fd, err := os.Open(filename); err != nil {
					errchan <- err
					return
				} else {
					l.fd.Close()
					l.fd = fd
					l.reader.Reset(l.fd)
					l.curReadPos = 0
				}
			}
			if (event.Op & fsnotify.Write == fsnotify.Write) && (event.Name == filename) {
				// If filename is written to, check if truncated (& handle accordingly), then read
				if err := l.handleFsnotifyWrite(); err != nil && err != io.EOF {
					errchan <- err
					return
				}
			}
		}
	}
}

func readLogFile(path string, callback func(string) error) error {
	dir := filepath.Dir(path)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	errchan := make(chan error)
	go tail(watcher, path, errchan, callback)

	if err = watcher.Add(dir); err != nil {
		return err
	}

	err = <-errchan
	return err
}
