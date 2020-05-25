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

// LogWriter ...
//

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

type LogWriter struct {
	sync.Mutex

	// the log group to which the log stream belongs
	logGroup string

	// the log stream to which we will write
	logStream string

	buf []string

	// bufSize is the
	bufSize int

	// ticker is used to periodically flush the buffer
	ticker *time.Ticker

	started bool

	// scanErr holds any error that is returned by the internal scanner
	scanErr error

	// pw and pr (io.Pipe) are used to pipe input delivered to Write to the internal
	// bufio.Scanner which reads input in a linewise fashion
	pw *io.PipeWriter
	pr *io.PipeReader

	logsClient cloudwatchlogsiface.CloudWatchLogsAPI
}

func New(logGroup, logStream string, client Client) *LogWriter {
	pr, pw := io.Pipe()

	b := LogWriter{
		logGroup:   logGroup,
		logStream:  logStream,
		pw:         pw,
		pr:         pr,
		ticker:     time.NewTicker(2 * time.Second),
		logsClient: client,
	}

	go b.start()

	return &b
}

func (w *LogWriter) Write(data []byte) (int, error) {
	return w.pw.Write(data)
}

func (w *LogWriter) Close() error {
	w.stop()
	return w.scanErr
}

func (w *LogWriter) Flush() error {
	return nil
}

func (w *LogWriter) start() {
	w.started = true

	go w.readLines()
}

func (w *LogWriter) readLines() {
	sc := bufio.NewScanner(w.pr)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		l := sc.Text()
		w.buf = append(w.buf, l)
	}

	w.scanErr = sc.Err()
}

func (w *LogWriter) stop() {
	w.ticker.Stop()
}
