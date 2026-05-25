package skilpel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

func progressLogger(w io.Writer, format string) *slog.Logger {
	return slog.New(progressHandler(w, format))
}

func progressHandler(w io.Writer, format string) slog.Handler {
	if usesPrettyProgress(w, format) {
		return newPrettyProgressHandlerWithLive(w, usesLiveProgress(w, format))
	}
	return structuredHandler(w)
}

func usesPrettyProgress(w io.Writer, format string) bool {
	if format == "pretty" {
		return true
	}
	if format != "auto" {
		return false
	}
	file, ok := w.(*os.File)
	return ok && isTerminal(file)
}

func usesLiveProgress(w io.Writer, format string) bool {
	file, ok := w.(*os.File)
	return ok && isTerminal(file) && usesPrettyProgress(w, format)
}

func structuredLogger(w io.Writer) *slog.Logger {
	return slog.New(structuredHandler(w))
}

func structuredHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return attr
			}
			switch attr.Key {
			case slog.LevelKey:
				attr.Key = "severity"
			case slog.MessageKey:
				attr.Key = "message"
			}
			return attr
		},
	})
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

type prettyProgressHandler struct {
	w     io.Writer
	state *prettyProgressState
	attrs []slog.Attr
	group string
}

type prettyProgressState struct {
	mu          sync.Mutex
	total       int
	done        int
	warnings    bool
	evalsHeader bool
	live        bool
	liveLine    bool
	closed      bool
	spin        int
	stop        chan struct{}
}

func newPrettyProgressHandler(w io.Writer) *prettyProgressHandler {
	return newPrettyProgressHandlerWithLive(w, false)
}

func newPrettyProgressHandlerWithLive(w io.Writer, live bool) *prettyProgressHandler {
	return &prettyProgressHandler{w: w, state: &prettyProgressState{live: live}}
}

func (h *prettyProgressHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *prettyProgressHandler) Close() {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	h.stopLiveStatus()
}

func (h *prettyProgressHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := map[string]any{}
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	switch attrs["event"] {
	case "run_started":
		h.state.total = attrInt(attrs, "evals")
		h.state.done = 0
		h.state.warnings = false
		h.state.evalsHeader = false
		h.state.closed = false
		_, err := fmt.Fprintf(
			h.w,
			"\n%sValidating skills: %s%s\n\n%sConfiguration%s\n  %sℹ %d skill%s, %d eval%s%s\n  %sℹ provider=%s target=%s judge=%s without_skill=%t%s\n",
			colorBold,
			attrString(attrs, "root"),
			colorReset,
			colorBold,
			colorReset,
			colorCyan,
			attrInt(attrs, "skills"),
			pluralS(attrInt(attrs, "skills")),
			h.state.total,
			pluralS(h.state.total),
			colorReset,
			colorCyan,
			attrString(attrs, "provider"),
			attrString(attrs, "target"),
			attrString(attrs, "judge"),
			attrBool(attrs, "baseline"),
			colorReset,
		)
		h.startLiveStatus()
		return err
	case "warning":
		h.clearLiveLine()
		if !h.state.warnings {
			if _, err := fmt.Fprintf(h.w, "\n%sWarnings%s\n", colorBold, colorReset); err != nil {
				return err
			}
			h.state.warnings = true
		}
		_, err := fmt.Fprintf(
			h.w,
			"  %s⚠ %s: %s%s\n",
			colorYellow,
			attrString(attrs, "skill"),
			attrString(attrs, "warning"),
			colorReset,
		)
		h.drawLiveLine()
		return err
	case "eval_completed":
		h.clearLiveLine()
		if !h.state.evalsHeader {
			if _, err := fmt.Fprintf(h.w, "\n%sEvals%s\n", colorBold, colorReset); err != nil {
				return err
			}
			h.state.evalsHeader = true
		}
		h.state.done++
		icon, color := levelIcon(true)
		if attrInt(attrs, "failed") > 0 {
			icon, color = levelIcon(false)
		}
		_, err := fmt.Fprintf(
			h.w,
			"  %s%s %s / %s:%s %d passed, %d failed\n    %swith:%s %s%s%s\n    %swithout:%s %s%s%s\n    %sdelta:%s %s%s%s\n",
			color,
			icon,
			attrString(attrs, "rel_path"),
			displayEval(attrs),
			colorReset,
			attrInt(attrs, "passed"),
			attrInt(attrs, "failed"),
			colorCyan,
			colorReset,
			colorBold,
			formatRate(attrFloat(attrs, "with_skill_pass_rate")),
			colorReset,
			colorCyan,
			colorReset,
			colorBold,
			formatOptionalRate(attrs, "without_skill_pass_rate"),
			colorReset,
			colorCyan,
			colorReset,
			colorBold,
			formatOptionalRate(attrs, "delta"),
			colorReset,
		)
		h.drawLiveLine()
		return err
	case "run_completed":
		h.stopLiveStatus()
		return nil
	default:
		h.clearLiveLine()
		_, err := fmt.Fprintf(h.w, "%s\n", record.Message)
		h.drawLiveLine()
		return err
	}
}

func progressBar(done, total int) string {
	const width = 10
	if total <= 0 {
		return "[----------]"
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("─", width-filled) + "]"
}

func (h *prettyProgressHandler) startLiveStatus() {
	if !h.state.live || h.state.stop != nil {
		return
	}
	h.state.stop = make(chan struct{})
	_, _ = fmt.Fprintln(h.w)
	h.drawLiveLine()
	go func(state *prettyProgressState, w io.Writer, stop <-chan struct{}) {
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				state.mu.Lock()
				if state.closed {
					state.mu.Unlock()
					return
				}
				state.spin++
				writeLiveLine(w, state)
				state.mu.Unlock()
			case <-stop:
				return
			}
		}
	}(h.state, h.w, h.state.stop)
}

func (h *prettyProgressHandler) stopLiveStatus() {
	if !h.state.live {
		return
	}
	if h.state.stop != nil {
		close(h.state.stop)
		h.state.stop = nil
	}
	h.state.closed = true
	h.clearLiveLine()
}

func (h *prettyProgressHandler) clearLiveLine() {
	if !h.state.live || !h.state.liveLine {
		return
	}
	_, _ = fmt.Fprint(h.w, "\r\033[2K")
	h.state.liveLine = false
}

func (h *prettyProgressHandler) drawLiveLine() {
	if !h.state.live || h.state.closed {
		return
	}
	writeLiveLine(h.w, h.state)
}

func writeLiveLine(w io.Writer, state *prettyProgressState) {
	frame := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[state.spin%10]
	_, _ = fmt.Fprintf(w, "\r\033[2K  %s%s%s %s [%d/%d] running", colorCyan, frame, colorReset, progressBar(state.done, state.total), state.done, state.total)
	state.liveLine = true
}

func (h *prettyProgressHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(slicesClone(h.attrs), attrs...)
	return &next
}

func (h *prettyProgressHandler) WithGroup(name string) slog.Handler {
	next := *h
	next.group = name
	return &next
}

func slicesClone[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]T, len(values))
	copy(cloned, values)
	return cloned
}

func displayEval(attrs map[string]any) string {
	if evalID := attrString(attrs, "eval_id"); evalID != "" {
		return evalID
	}
	if evalName := attrString(attrs, "eval_name"); evalName != "" {
		return evalName
	}
	return attrString(attrs, "eval_slug")
}

func attrString(attrs map[string]any, key string) string {
	if value, ok := attrs[key].(string); ok {
		return value
	}
	return ""
}

func attrBool(attrs map[string]any, key string) bool {
	if value, ok := attrs[key].(bool); ok {
		return value
	}
	return false
}

func attrInt(attrs map[string]any, key string) int {
	switch value := attrs[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	default:
		return 0
	}
}

func attrFloat(attrs map[string]any, key string) float64 {
	switch value := attrs[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func formatRate(value float64) string {
	return fmt.Sprintf("%.0f%%", value*100)
}

func formatOptionalRate(attrs map[string]any, key string) string {
	if _, ok := attrs[key]; !ok {
		return "-"
	}
	return formatRate(attrFloat(attrs, key))
}

type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var joined error
	for _, handler := range h {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make(multiHandler, len(h))
	for i, handler := range h {
		next[i] = handler.WithAttrs(attrs)
	}
	return next
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	next := make(multiHandler, len(h))
	for i, handler := range h {
		next[i] = handler.WithGroup(name)
	}
	return next
}
