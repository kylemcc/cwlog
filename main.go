// Copyright 2020 Kyle McCullough. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/genuinetools/pkg/cli"
	"github.com/kylemcc/cwlog/writer"
)

var (
	tee bool

	logGroup  string
	logStream string
)

func main() {
	p := cli.NewProgram()
	p.Name = "cwlog"
	p.Description = `A tee(1)-like command for piping output to CloudWatch Logs.

This program will read line-oriented data from standard input and send
log events to CloudWatch Logs. If the specified log group and/or log stream
do not exist, cwlog will attempt to create them. CloudWatch Logs also
requires a sequence token for existing streams that already contain log
events. If an existing stream is specified, cwlog will automatically
retrieve the next sequence token.

The execution of this program is optimized for the scenario where it is
invoked with an existing-but-empty log stream. It first attempts to write to
the specified log stream, and only tries to create the log group or log stream
if it receives an error.`

	p.FlagSet = flag.NewFlagSet("global", flag.ExitOnError)
	p.FlagSet.BoolVar(&tee, "tee", true, "If true, output will be copied to stdout")
	p.FlagSet.BoolVar(&tee, "t", true, "If true, output will be copied to stdout")
	p.FlagSet.StringVar(&logGroup, "log-group", os.Getenv("CWLOG_LOG_GROUP"), "(Required) The name of the log group where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_GROUP=]")
	p.FlagSet.StringVar(&logGroup, "g", os.Getenv("CWLOG_LOG_GROUP"), "(Required) The name of the log group where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_GROUP=]")
	p.FlagSet.StringVar(&logStream, "log-stream", os.Getenv("CWLOG_LOG_STREAM"), "(Required) The name of the log stream where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_STREAM=]")
	p.FlagSet.StringVar(&logStream, "s", os.Getenv("CWLOG_LOG_STREAM"), "(Required) The name of the log stream where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_STREAM=]")

	p.Before = func(ctx context.Context) error {
		if logGroup == "" || logStream == "" {
			p.FlagSet.Usage()
			return fmt.Errorf("log-group and log-stream are required")
		}
		return nil
	}

	p.Action = func(ctx context.Context, args []string) error {
		if err := run(logGroup, logStream, getSource(tee)); err != nil {
			return fmt.Errorf("error: failed to write logs: %v", err)
		}
		return nil
	}

	p.Run()
}

func run(logGroup, logStream string, src io.Reader) error {
	sess := session.Must(session.NewSession())
	client := cloudwatchlogs.New(sess)
	w := writer.New(logGroup, logStream, client)

	_, err := io.Copy(w, src)
	if err != nil {
		return fmt.Errorf("error writing logs: %w", err)
	}

	// flush any remaining data in the buffer
	return w.Close()
}

func getSource(tee bool) io.Reader {
	if tee {
		return io.TeeReader(os.Stdin, os.Stdout)
	}
	return os.Stdin
}
