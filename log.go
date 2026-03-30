package outrunner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// SimpleHandler outputs logs in a human-readable format:
//
//	2026-03-30 21:08:31 INFO Loaded config runners=1
type SimpleHandler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Leveler
	group string
	attrs []slog.Attr
}

func NewSimpleHandler(w io.Writer, level slog.Leveler) *SimpleHandler {
	return &SimpleHandler{
		w:     w,
		mu:    &sync.Mutex{},
		level: level,
	}
}

func (h *SimpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *SimpleHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	ts := r.Time.Format("2006-01-02 15:04:05")
	_, _ = fmt.Fprintf(h.w, "%s %s %s", ts, r.Level.String(), r.Message)

	// Pre-set attrs from WithAttrs/WithGroup
	for _, a := range h.attrs {
		h.writeAttr(a)
	}

	// Per-record attrs
	r.Attrs(func(a slog.Attr) bool {
		h.writeAttr(a)
		return true
	})

	_, _ = fmt.Fprintln(h.w)
	return nil
}

func (h *SimpleHandler) writeAttr(a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	key := a.Key
	if h.group != "" {
		key = h.group + "." + key
	}
	_, _ = fmt.Fprintf(h.w, " %s=%v", key, a.Value)
}

func (h *SimpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SimpleHandler{
		w:     h.w,
		mu:    h.mu,
		level: h.level,
		group: h.group,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *SimpleHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &SimpleHandler{
		w:     h.w,
		mu:    h.mu,
		level: h.level,
		group: g,
		attrs: append([]slog.Attr{}, h.attrs...),
	}
}
