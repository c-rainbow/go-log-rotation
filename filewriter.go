package main

import (
	"bufio"
	"log"
	"os"
)

func exists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

type LogFileWriter struct {
	file   *os.File      // File handle
	writer *bufio.Writer // Buffered writer to the file
}

func (w *LogFileWriter) isOpen() bool {
	return w.writer != nil
}

/*
Get file handle from the input timestamp
*/
func (w *LogFileWriter) openLogFile(filepath string) error {
	if w.isOpen() {
		w.Close()
	}

	var file *os.File
	var err error

	// Check for existing file
	if !exists(filepath) {
		file, err = os.Create(filepath)
	} else {
		// TODO: os.O_CREATE panicked with permission error
		file, err = os.OpenFile(filepath, os.O_APPEND, os.ModeAppend)
	}

	if err != nil {
		log.Println("Failed to create or open log file", err.Error())
		return err
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	return nil
}

// Return the current filename. If closed, return empty string.
func (w *LogFileWriter) Name() string {
	if !w.isOpen() {
		return ""
	}
	return w.file.Name()
}

func (w *LogFileWriter) Close() error {
	if !w.isOpen() { // Do nothing if already closed
		return nil
	}

	defer func() { // De-reference them anyway
		w.writer = nil
		w.file = nil
	}()

	w.writer.Flush()
	if err := w.file.Close(); err != nil {
		return err
	}
	return nil
}

func (w *LogFileWriter) Write(s string) error {
	if !w.isOpen() {
		return ErrClosed
	}
	_, err := w.writer.WriteString(s)
	return err
}

func (w *LogFileWriter) Flush() error {
	if !w.isOpen() {
		return ErrClosed
	}

	return w.writer.Flush()
}
