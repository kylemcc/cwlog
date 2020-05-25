// cwlog is a small utility for sending log data to CloudWatch Logs. Given a
// log group and stream name, cwlogger will read lines from standard input and
// attempt to send those logs to CloudWatch Logs.
//
// If the log group or log stream do not exist, cwlogger will attempt to create
// them.
//
// This program behaves like tee(1) - it will copy input to standard output in
// addition to sending to CloudWatch Logs.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kylemcc/cwlog/writer"
)

var (
	tee bool
)

func init() {
	flag.BoolVar(&tee, "tee", true, "If true, output will be copied to stdout")
	flag.BoolVar(&tee, "t", true, "If true, output will be copied to stdout")
	flag.Usage = usage
}

func main() {
	flag.Parse()

	if len(os.Args) < 3 {
		usage()
	}

	logGroup := os.Args[1]
	logStream := os.Args[2]

	if err := run(logGroup, logStream, getSource(tee)); err != nil {
		fmt.Printf("error: failed to write logs: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf(`Usage: %s [options] log_group_name log_stream_name

A tee-like command for piping output to CloudWatch Logs.

Options:
`, os.Args[0])

	flag.PrintDefaults()

	fmt.Printf(`
Arguments:

	log_group_name: (Required) The name of the log group where logs should be sent. The
		program will attempt to create this if it does not exist.

	log_stream_name: (Required) The name of the log stream where logs should be sent. The
		program will attempt to create this if it does not exist.
`)

	os.Exit(1)
}

func run(logGroup, logStream string, src io.Reader) error {
	w := writer.New(logGroup, logStream)

	_, err := io.Copy(w, src)
	if err != nil {
		return fmt.Errorf("error writing logs: %w", err)
	}

	// flush any remaing data in the buffer
	return w.Close()
}

func getSource(tee bool) io.Reader {
	if tee {
		return io.TeeReader(os.Stdin, os.Stdout)
	}
	return os.Stdin
}
