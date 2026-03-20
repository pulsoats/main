package logger

import (
	"fmt"

	"github.com/pulsoats/core/lib/logx"
	"github.com/rs/zerolog"
)

type zerologAdapter struct {
	logger zerolog.Logger
}

// AsLogx converts zerolog.Logger to logx.Logger.
func AsLogx(z zerolog.Logger) logx.Logger {
	return &zerologAdapter{logger: z}
}

func (l *zerologAdapter) Debug(msg string, kv ...any) { l.write(l.logger.Debug(), msg, kv...) }
func (l *zerologAdapter) Info(msg string, kv ...any)  { l.write(l.logger.Info(), msg, kv...) }
func (l *zerologAdapter) Warn(msg string, kv ...any)  { l.write(l.logger.Warn(), msg, kv...) }
func (l *zerologAdapter) Error(msg string, kv ...any) { l.write(l.logger.Error(), msg, kv...) }

func (l *zerologAdapter) write(evt *zerolog.Event, msg string, kv ...any) {
	if len(kv) > 0 {
		fields := make(map[string]any, len(kv)/2)
		for i := 0; i < len(kv); i += 2 {
			var (
				key string
				val any
			)
			if k, ok := kv[i].(string); ok {
				key = k
			} else {
				key = fmt.Sprintf("field_%d", i/2)
			}
			if i+1 < len(kv) {
				val = kv[i+1]
			}
			fields[key] = val
		}
		evt.Fields(fields)
	}

	evt.Msg(msg)
}
