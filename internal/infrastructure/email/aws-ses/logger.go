package aws_ses

import (
	"fmt"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

type AwsLogger struct {
	log *slog.Logger
}

func NewAWSLogger(log *slog.Logger) AwsLogger {
	return AwsLogger{log: log}
}

func (l AwsLogger) Logf(class logging.Classification, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if class == logging.Warn {
		l.log.Warn(msg)
	} else {
		l.log.Debug(msg)
	}

}
