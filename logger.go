package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/emirpasic/gods/queues/priorityqueue"
)

var (
	ErrClosed = errors.New("Logger is already closed")
)

type element struct {
	message   string    // message content
	timestamp time.Time // Epoch timestamp of the message
}

/*
A simple comparator function of elements for the priority queue
*/
func byTimestampAndId(i1 interface{}, i2 interface{}) int {
	e1, e2 := i1.(element), i2.(element)
	if e1.timestamp.Before(e2.timestamp) {
		return -1
	}
	if e1.timestamp.After(e2.timestamp) {
		return 1
	}

	// Compare raw message strings for tie-breaking
	return strings.Compare(e1.message, e2.message)
}

func getEpochHour(timestamp *time.Time) *time.Time {
	utc := timestamp.UTC()
	truncated := time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), 0, 0, 0, time.UTC)
	return &truncated
}

/*
 */
type RotatingFileLogger struct {
	mutex    sync.Mutex
	isOpen   bool
	pq       *priorityqueue.Queue
	baseDir  string // Base dir path to create files
	prefix   string // Prefix of filenames to be created
	file     *os.File
	writer   *bufio.Writer // Buffered writer to the file
	interval time.Duration // Interval to flush logs to file
	ticker   *time.Ticker  // Ticker that runs every interval
}

func New(baseDir string, prefix string, interval time.Duration) *RotatingFileLogger {
	logger := RotatingFileLogger{
		isOpen:   true,
		pq:       priorityqueue.NewWith(byTimestampAndId),
		baseDir:  baseDir,
		prefix:   prefix,
		writer:   nil,
		interval: interval,
		ticker:   nil,
	}
	go logger.start()

	return &logger
}

func (logger *RotatingFileLogger) AddMessage(message string, timestamp time.Time) {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()

	if !logger.isOpen {
		return
	}

	logger.pq.Enqueue(element{message: message, timestamp: timestamp})
}

func (logger *RotatingFileLogger) Close() error {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()

	logger.ticker.Stop()
	logger.flushToFile()
	return logger.file.Close()
}

// It is assumed that the mutex is already locked.
func (logger *RotatingFileLogger) flushToFile() error {
	for logger.pq.Size() > 0 {
		dequeued, _ := logger.pq.Dequeue()
		e := dequeued.(element)

		logFilePath := logger.getLogFilePath(&e.timestamp)
		// New epoch hour. Close the previous log file and create a new one
		if logFilePath != logger.file.Name() {
			if logger.writer != nil {
				logger.writer.Flush()
				logger.writer = nil
			}

			file, err := logger.openLogFile(logFilePath)
			if err != nil {
				return err
			}
			logger.file = file
			logger.writer = bufio.NewWriter(file)
		}

		if _, err := logger.writer.WriteString(e.message + "\n"); err != nil {
			return err
		}

	}

	return logger.writer.Flush()
}

func (logger *RotatingFileLogger) start() {
	if err := os.MkdirAll(logger.baseDir, fs.FileMode(os.O_RDWR)); err != nil {
		log.Fatalln("Cannot create a base dir", logger.baseDir)
		return
	}

	logger.ticker = time.NewTicker(logger.interval)
	for {
		select {
		case <-logger.ticker.C:
			func() {
				logger.mutex.Lock()
				defer logger.mutex.Unlock()
				logger.flushToFile()
			}()
		}
	}
}

func (logger *RotatingFileLogger) getLogFilePath(ts *time.Time) string {
	name := fmt.Sprintf("%4d-%2d-%2dT%2d00.log", ts.Year(), ts.Month(), ts.Day(), ts.Hour())
	filename := path.Join(logger.baseDir, logger.prefix, name)
	return filename
}

/*
Get file handle from the input timestamp
*/
func (logger *RotatingFileLogger) openLogFile(filepath string) (*os.File, error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_APPEND, fs.ModeAppend)
	if err != nil {
		log.Fatalln("Cannot open log file", filepath)
		return nil, err
	}
	return file, nil
}
