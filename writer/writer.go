// Package writer provides an io.Writer interface to CloudWatch Logs
package writer

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
)

const (

	// maxSize is the maximum number of bytes in a single cloudwatch
	// log batch. The batch size is calculated by counting the number
	// of bytes in each UTF-8-encoded event + 26 bytes per event
	//
	// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	maxSize = 1_048_576

	// maxEvents is the maximum number of events is a single cloudwatch
	// log batch.
	//
	// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	maxEvents = 10_000

	// eventSize is the static size of each event object excluding the message text. This is used
	// to calculate the size of each log batch.
	eventSize = 26

	// maxRetries is the max number of times a cloudwatch operation will be attempted
	// before giving up
	maxRetries = 5
)

// now returns the current timestamp. it's a variable here so we can swap it out for testing
var now = func() int64 {
	return time.Now().UnixNano() / 1000000
}

// Client is a CloudWatch Logs client
type Client cloudwatchlogsiface.CloudWatchLogsAPI

// LogWriter provides an io.Writer interface to CloudWatch Logs
//
// The zero-value is not usable. New should be used to construct
// a new LogWriter
type LogWriter struct {
	sync.Mutex

	// the log group to which the log stream belongs
	logGroup string

	// the log stream to which we will write
	logStream string

	// buf holds pending log events that have not yet been written to CloudWatch Logs
	buf []*cloudwatchlogs.InputLogEvent

	bufSize int

	// ticker is used to periodically flush the buffer
	ticker *time.Ticker

	// scanErr will receieve the return value of the internal scanner
	scanErr chan error

	// flushErr holds any error encountered while attempting to write
	// logs to CloudWatch Logs. If the writer encounters an error,
	// and exhausts retry attepmts, it will not continue trying to write logs
	flushErr error

	// close will receive a message when the writer is closed
	closed chan struct{}

	// signalFlush will receive a message when the writer wants to trigger a Flush operation
	signalFlush chan struct{}

	// pw and pr (io.Pipe) are used to pipe input delivered to Write to the internal
	// bufio.Scanner which reads input in a linewise fashion
	pw *io.PipeWriter
	pr *io.PipeReader

	// sequenceToken is token returned by cloudwatch logs after a PutLogEvents request. This
	// token is required on all calls to PutLogEvents except the first call to a newly created
	// log stream.
	sequenceToken string

	logsClient cloudwatchlogsiface.CloudWatchLogsAPI
}

// New constructs and returns a new LogWriter
func New(logGroup, logStream string, client Client) *LogWriter {
	pr, pw := io.Pipe()

	b := LogWriter{
		logGroup:    logGroup,
		logStream:   logStream,
		pw:          pw,
		pr:          pr,
		ticker:      time.NewTicker(2 * time.Second),
		scanErr:     make(chan error),
		closed:      make(chan struct{}),
		signalFlush: make(chan struct{}),
		logsClient:  client,
	}

	go b.start()

	return &b
}

// Write implements io.Writer
func (w *LogWriter) Write(data []byte) (int, error) {
	return w.pw.Write(data)
}

// Close implements io.Closer. This method will stop the writer and flush
// any buffered log events
func (w *LogWriter) Close() error {
	w.pw.Close()
	w.stop()

	if err := <-w.scanErr; err != nil {
		return err
	}

	return w.flushAll()
}

// Flush writes any buffered log events to CloudWatch Logs
func (w *LogWriter) Flush() error {
	if w.flushErr != nil {
		return w.flushErr

	}

	w.Lock()
	defer w.Unlock()

	if len(w.buf) == 0 {
		return nil
	}

	events := w.drainBuffer()

	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  &w.logGroup,
		LogStreamName: &w.logStream,
	}

	err := retry(func() error {
		if w.sequenceToken != "" {
			input.SetSequenceToken(w.sequenceToken)
		}

		resp, err := w.logsClient.PutLogEvents(input)
		if err != nil {
			return w.handleError(err)
		}

		w.sequenceToken = *resp.NextSequenceToken
		return nil
	})

	w.flushErr = err
	return err
}

func (w *LogWriter) handleError(err error) error {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudwatchlogs.ErrCodeDataAlreadyAcceptedException:
			// data was already accepted
			if e, ok := err.(*cloudwatchlogs.DataAlreadyAcceptedException); ok {
				w.sequenceToken = *e.ExpectedSequenceToken
			}
			return nil
		case cloudwatchlogs.ErrCodeInvalidSequenceTokenException:
			if e, ok := err.(*cloudwatchlogs.InvalidSequenceTokenException); ok {
				w.sequenceToken = *e.ExpectedSequenceToken
			}
		case cloudwatchlogs.ErrCodeResourceNotFoundException:
			if err := w.createLogStream(); err != nil {
				return err
			}
		}
	}
	return err
}

func (w *LogWriter) createLogStream() error {
	//TODO
	return fmt.Errorf("not implemented")
}

func (w *LogWriter) drainBuffer() []*cloudwatchlogs.InputLogEvent {
	var (
		size   int
		cnt    int
		events []*cloudwatchlogs.InputLogEvent
	)

	for _, e := range w.buf {
		if size > maxSize || len(events) >= maxEvents {
			break
		}

		size += len(*e.Message) + eventSize
		events = append(events, e)
		cnt++
	}

	w.buf = w.buf[cnt:]
	w.bufSize -= size

	return events
}

func (w *LogWriter) start() {
	go w.readLines()
	go w.periodicFlush()
}

func (w *LogWriter) readLines() {
	sc := bufio.NewScanner(w.pr)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		w.appendEvent(sc.Text())
	}

	w.scanErr <- sc.Err()
}

func (w *LogWriter) appendEvent(text string) {
	if text == "" {
		return
	}

	w.Lock()
	defer w.Unlock()
	w.buf = append(w.buf, &cloudwatchlogs.InputLogEvent{
		Message:   &text,
		Timestamp: aws.Int64(now()),
	})

	w.bufSize += len(text) + 26
}

func (w *LogWriter) periodicFlush() {
	for {
		select {
		case <-w.ticker.C:
			w.Flush()
		case <-w.signalFlush:
			w.Flush()
		case <-w.closed:
			return
		}
	}
}

func (w *LogWriter) stop() {
	w.ticker.Stop()
	w.closed <- struct{}{}
}

func (w *LogWriter) flushAll() error {
	for len(w.buf) > 0 {
		if err := w.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func retry(f func() error) error {
	var (
		cnt int
		err error
	)

	for cnt < maxRetries {
		if cnt > 0 {
			time.Sleep(time.Duration(cnt) * 100 * time.Millisecond)
		}

		if err = f(); err == nil {
			return nil
		}

		cnt++
	}

	return err
}
