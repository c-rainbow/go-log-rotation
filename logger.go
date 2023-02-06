package main

import (
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
	baseDir  string
	prefix   string
	pq       *priorityqueue.Queue
	interval time.Duration // Interval to flush logs to file
	ticker   *time.Ticker  // Ticker that runs every interval
	closeCh  chan (bool)
	writer   LogFileWriter // Abstraction of file writer
}

func NewLogger(baseDir string, prefix string, interval time.Duration) *RotatingFileLogger {
	if err := os.MkdirAll(baseDir, fs.FileMode(os.O_RDWR)); err != nil {
		log.Fatalln("Cannot create a base dir", baseDir)
	}

	logger := RotatingFileLogger{
		isOpen:   true,
		baseDir:  baseDir,
		prefix:   prefix,
		pq:       priorityqueue.NewWith(byTimestampAndId),
		interval: interval,
		ticker:   nil,
		closeCh:  make(chan bool),
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
	logger.closeCh <- true
	return logger.writer.Close()
}

func (logger *RotatingFileLogger) FlushToFile() error {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	return logger.flushToFile()
}

// It is assumed that the mutex is already locked.
func (logger *RotatingFileLogger) flushToFile() error {
	for logger.pq.Size() > 0 {
		dequeued, _ := logger.pq.Dequeue()
		e := dequeued.(element)

		logFilename := logger.getLogFilename(&e.timestamp)
		// New epoch hour. Close the previous log file and create a new one
		if logFilename != logger.writer.Name() {
			logFilePath := path.Join(logger.baseDir, logFilename)
			err := logger.writer.openLogFile(logFilePath)
			if err != nil {
				return err
			}
		}

		return logger.writer.Write(e.message + "\n")
	}

	return logger.writer.Flush()
}

func (logger *RotatingFileLogger) getLogFilename(ts *time.Time) string {
	filename := fmt.Sprintf("%s%04d-%02d-%02dT%02d00.log", logger.prefix, ts.Year(), ts.Month(), ts.Day(), ts.Hour())
	return filename
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
			logger.FlushToFile()
		case <-logger.closeCh:
			break
		}
	}
}
