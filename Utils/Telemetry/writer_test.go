package Telemetry

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS/testHandler"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type writerTester struct {
	suite.Suite
	config *TelemetryConfiguration
	aws    *testHandler.TestAwsHandler
}

func (s *writerTester) SetupSuite() {
	s.aws = testHandler.NewTestAwsHandler()
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
	fileName, err := writer.createFilesFromMessages(&messages)
	s.NoError(err)
	s.Empty(fileName)

	messages = make([]telemetryLogMsg, 1)
	msg := NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[0] = msg
	fileName, err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
	s.NotEmpty(fileName)
	shouldFileName := fmt.Sprintf("%s/log--%s--%s.json.gz",
		writer.fileLocationPrefix,
		messages[0].Timestamp.Format(TelemetryTimeZFormat),
		messages[0].Timestamp.Format(TelemetryTimeZFormat))

	s.Equal(shouldFileName, fileName)

	messages = make([]telemetryLogMsg, 2)
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-1)*time.Minute),
	)
	messages[0] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[1] = msg
	fileName, err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
	s.NotEmpty(fileName)
	shouldFileName = fmt.Sprintf("%s/log--%s--%s.json.gz",
		writer.fileLocationPrefix,
		messages[0].Timestamp.Format(TelemetryTimeZFormat),
		messages[1].Timestamp.Format(TelemetryTimeZFormat))

	s.Equal(shouldFileName, fileName)

	messages = make([]telemetryLogMsg, 3)
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-2)*time.Minute),
	)
	messages[0] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC().Add(time.Duration(-1)*time.Minute),
	)
	messages[1] = msg
	msg = NewTelemetryMsg(
		telemetryLogEventInfo,
		"foo",
		"someclient",
		"some message",
		"some hostid",
		time.Now().Round(time.Millisecond).UTC(),
	)
	messages[2] = msg
	fileName, err = writer.createFilesFromMessages(&messages)
	s.NoError(err)
	s.NotEmpty(fileName)
	shouldFileName = fmt.Sprintf("%s/log--%s--%s.json.gz",
		writer.fileLocationPrefix,
		messages[0].Timestamp.Format(TelemetryTimeZFormat),
		messages[2].Timestamp.Format(TelemetryTimeZFormat))

	s.Equal(shouldFileName, fileName)
}

func (s *writerTester) TestClientRegex() {
	message := "NchoDC28E565X072CX46B1XBF205:us-east-1:44211d8c-caf6-4b17-80cf-72febe0ebb2d:12345:vgifyyyTOxF2rvS1/ActiveSync: exiting LongPoll"
	s.False(deviceClientContextRegexp.MatchString(message), "should have matched deviceClientContextRegexp")
	s.True(deviceClientContextProtocolRegexp.MatchString(message), "should have matched deviceClientContextProtocolRegexp")
	s.True(clientIdRegex.MatchString(message), "should have matched clientIdRegex")

	message = "NchoDC28E565X072CX46B1XBF205:us-east-1:44211d8c-caf6-4b17-80cf-72febe0ebb2d:12345:vgifyyyTOxF2rvS1: exiting LongPoll"
	s.True(deviceClientContextRegexp.MatchString(message), "should have matched deviceClientContextRegexp")
	s.False(deviceClientContextProtocolRegexp.MatchString(message), "should have matched deviceClientContextProtocolRegexp")
	s.True(clientIdRegex.MatchString(message), "should have matched clientIdRegex")
}
