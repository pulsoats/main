package logger

import (
	"context"
	"log/slog"

	"github.com/rs/zerolog"
)

// NewSlog wraps a zerolog.Logger as a *slog.Logger.
func NewSlog(z zerolog.Logger) *slog.Logger {
	return slog.New(&zerologHandler{logger: z})
}

type zerologHandler struct {
	logger zerolog.Logger
	attrs  []slog.Attr
	group  string
}

func (h *zerologHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.logger.GetLevel() <= toZerologLevel(level)
}

func (h *zerologHandler) Handle(_ context.Context, r slog.Record) error {
	evt := h.logger.WithLevel(toZerologLevel(r.Level))
	if !evt.Enabled() {
		return nil
	}

	for _, a := range h.attrs {
		addAttr(evt, a, h.group)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(evt, a, h.group)
		return true
	})

	evt.Msg(r.Message)
	return nil
}

func (h *zerologHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)
	return &zerologHandler{logger: h.logger, attrs: merged, group: h.group}
}

func (h *zerologHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &zerologHandler{logger: h.logger, attrs: h.attrs, group: g}
}

func addAttr(evt *zerolog.Event, a slog.Attr, group string) {
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		evt.Str(key, v.String())
	case slog.KindInt64:
		evt.Int64(key, v.Int64())
	case slog.KindUint64:
		evt.Uint64(key, v.Uint64())
	case slog.KindFloat64:
		evt.Float64(key, v.Float64())
	case slog.KindBool:
		evt.Bool(key, v.Bool())
	case slog.KindDuration:
		evt.Dur(key, v.Duration())
	case slog.KindTime:
		evt.Time(key, v.Time())
	case slog.KindGroup:
		for _, ga := range v.Group() {
			addAttr(evt, ga, key)
		}
	default:
		evt.Interface(key, v.Any())
	}
}

func toZerologLevel(l slog.Level) zerolog.Level {
	switch {
	case l >= slog.LevelError:
		return zerolog.ErrorLevel
	case l >= slog.LevelWarn:
		return zerolog.WarnLevel
	case l >= slog.LevelInfo:
		return zerolog.InfoLevel
	default:
		return zerolog.DebugLevel
	}
}
