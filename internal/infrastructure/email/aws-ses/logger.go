package aws_ses

import (
	"fmt"

	"github.com/aws/smithy-go/logging"
	"github.com/pulsoats/core/lib/logx"
)

type awsLogger struct {
	log logx.Logger
}

func (l awsLogger) Logf(class logging.Classification, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if class == logging.Warn {
		l.log.Warn(msg)
	} else {
		l.log.Debug(msg)
	}
}
