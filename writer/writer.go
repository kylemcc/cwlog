package writer

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
)

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

	// pw and pr (io.Pipe) are used to pipe input delivered to Write to the internal
	// bufio.Scanner which reads input in a linewise fashion
	pw *io.PipeWriter
	pr *io.PipeReader

	logsClient cloudwatchlogsiface.CloudWatchLogsAPI
}

// New constructs and returns a new LogWriter
func New(logGroup, logStream string, client Client) *LogWriter {
	pr, pw := io.Pipe()

	b := LogWriter{
		logGroup:   logGroup,
		logStream:  logStream,
		pw:         pw,
		pr:         pr,
		ticker:     time.NewTicker(2 * time.Second),
		scanErr:    make(chan error, 1),
		logsClient: client,
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
	return <-w.scanErr
}

// Flush writes any buffered log events to CloudWatch Logs
func (w *LogWriter) Flush() error {
	return nil
}

func (w *LogWriter) start() {
	go w.readLines()
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
	w.Lock()
	defer w.Unlock()
	w.buf = append(w.buf, &cloudwatchlogs.InputLogEvent{
		Message:   &text,
		Timestamp: aws.Int64(time.Now().UnixNano() / 1000000),
	})
}

func (w *LogWriter) stop() {
	w.ticker.Stop()
}
