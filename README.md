# cwlog

A tee(1)-like utility for piping logs to a CloudWatch Logs log stream. This is useful for sending standard output
of another command or application to cloudwatch logs.

## Usage

```sh
cwlog -  A tee(1)-like command for piping output to CloudWatch Logs.

This program will read line-oriented data from standard input and send
log events to CloudWatch Logs. If the specified log group and/or log stream
do not exist, cwlog will attempt to create them. CloudWatch Logs also
requires a sequence token for existing streams that already contain log
events. If an existing stream is specified, cwlog will automatically
retrieve the next sequence token.

The execution of this program is optimized for the scenario where it is
invoked with an existing-but-empty log stream. It first attempts to write to
the specified log stream, and only tries to create the log group or log stream
if it receives an error.

Usage: cwlog <command>

Flags:

  -g, --log-group   (Required) The name of the log group where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_GROUP=] (default: <none>)
  -s, --log-stream  (Required) The name of the log stream where logs should be sent. The program will attempt to create this if it does not exist. [env CWLOG_LOG_STREAM=] (default: <none>)
  -t, --tee         If true, output will be copied to stdout (default: true)

Commands:

  version  Show the version information.
```

First, configure your environment with credentials that have access to CloudWatch Logs. This tool uses the Go AWS SDK, which loads
credentials as described [here][1].

Next, pipe the log that should be sent to CloudWatch Logs to `cwlog`:

```sh
# Send a single command's output:
$ some-command-with-log-output | cwlog -g my-log-group -s my-log-stream

# Use environment variables to configure cwlog
$ export CWLOG_LOG_GROUP=my-log-group
$ export CWLOG_LOG_STREAM=my-log-stream
$ some-command | cwlog

# Use command grouping to capture multiple commands more efficiently:
$ { command-1; command-2; command-3 } | cwlog
```

[1]: https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#hdr-Credential_and_config_loading_order
