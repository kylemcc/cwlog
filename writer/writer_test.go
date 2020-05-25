package writer

import (
	"io"
	"reflect"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
)

type mockLogsAPI struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
	seq    int
	events []*cloudwatchlogs.InputLogEvent
}

// PutLogEvents implements cloudwatchlogsiface.CloudWatchLogsAPI
func (m *mockLogsAPI) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	m.events = append(m.events, input.LogEvents...)
	m.seq++
	return &cloudwatchlogs.PutLogEventsOutput{
		NextSequenceToken: aws.String(strconv.Itoa(m.seq)),
	}, nil
}

func newLogsCLientTest() *mockLogsAPI {
	return &mockLogsAPI{}
}

type mockStdin struct {
	cnt  int
	data [][]byte
}

func newTestInput(input [][]byte) io.Reader {
	return &mockStdin{data: input}
}

// Read implements io.Reader
func (m *mockStdin) Read(b []byte) (int, error) {
	if m.cnt >= len(m.data) {
		return 0, io.EOF
	}

	d := m.data[m.cnt]
	copy(b, d)
	m.cnt++
	return len(d), nil
}

func TestWriter(t *testing.T) {
	type Events = []*cloudwatchlogs.InputLogEvent

	cases := []struct {
		name     string
		input    io.Reader
		expected Events
	}{
		{
			"empty input",
			newTestInput(nil),
			nil,
		},
		{
			"single string",
			newTestInput([][]byte{
				[]byte("test input\n"),
			}),
			Events{
				{
					Message: aws.String("test input"),
				},
			},
		},
		{
			"multiple strings",
			newTestInput([][]byte{
				[]byte("test input\n"),
				[]byte("different test input\n"),
				[]byte("totally important log data\n"),
			}),
			Events{
				{
					Message: aws.String("test input"),
				},
				{
					Message: aws.String("different test input"),
				},
				{
					Message: aws.String("totally important log data"),
				},
			},
		},
		{
			"no ending newline",
			newTestInput([][]byte{
				[]byte("test input\n"),
				[]byte("no newline at the end"),
			}),
			Events{
				{
					Message: aws.String("test input"),
				},
				{
					Message: aws.String("no newline at the end"),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			logsClient := newLogsCLientTest()
			w := New("group", "stream", logsClient)

			_, err := io.Copy(w, c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err := w.Close(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Logf("buf=%#v", w.buf)

			if !reflect.DeepEqual(c.expected, logsClient.events) {
				t.Errorf("log events did not matchc: got=%#v want=%#v", logsClient.events, c.expected)
			}
		})
	}
}
