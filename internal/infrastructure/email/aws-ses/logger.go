package aws_ses

import (
	"fmt"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

type awsLogger struct {
	log *slog.Logger
}

func NewAWSLogger(log *slog.Logger) awsLogger {
	return awsLogger{log: log}
}

func (l awsLogger) Logf(class logging.Classification, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if class == logging.Warn {
		l.log.Warn(msg)
	} else {
		l.log.Debug(msg)
	}

}
