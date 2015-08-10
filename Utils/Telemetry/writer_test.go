package Telemetry

import (
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type writerTester struct {
	suite.Suite
	config *TelemetryConfiguration
	aws    *AWS.TestAwsHandler
}

func (s *writerTester) SetupSuite() {
	s.aws = AWS.NewTestAwsHandler()
}

func (s *writerTester) SetupTest() {
	s.config = NewTelemetryConfiguration()
}

func (s *writerTester) TearDownTest() {
}

func TestWriter(t *testing.T) {
	s := new(writerTester)
	suite.Run(t, s)
}

func (s *writerTester) TestNewTelemetryWriter() {
	writer, err := NewTelemetryWriter(s.config, s.aws, true)
	s.NoError(err)
	s.Nil(writer)

	s.config.FileLocationPrefix = "/tmp"
	writer, err = NewTelemetryWriter(s.config, s.aws, true)
	s.NoError(err)
	s.NotNil(writer)
}

func (s *writerTester) TestFileCreation() {
	s.config.FileLocationPrefix = "/tmp"
	writer, err := NewTelemetryWriter(s.config, s.aws, true)
	s.NoError(err)
	s.NotNil(writer)

	messages := make([]telemetryLogMsg, 0)
	err = writer.createFilesFromMessages(&messages)
	s.NoError(err)

	messages = make([]telemetryLogMsg, 1)
	msg := NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[0] = msg
	err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
	messages = make([]telemetryLogMsg, 2)
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-1)*time.Minute),
	)
	messages[0] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[1] = msg
	err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
	messages = make([]telemetryLogMsg, 3)
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-2)*time.Minute),
	)
	messages[0] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-1)*time.Minute),
	)
	messages[1] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"some message",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[2] = msg
	err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
}

