package skilpel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

func progressLogger(w io.Writer, format string) *slog.Logger {
	return slog.New(progressHandler(w, format))
}

func progressHandler(w io.Writer, format string) slog.Handler {
	switch format {
	case "pretty":
		return newPrettyProgressHandler(w)
	case "auto":
		if file, ok := w.(*os.File); ok && isTerminal(file) {
			return newPrettyProgressHandler(w)
		}
	}
	return structuredHandler(w)
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
	mu    sync.Mutex
	total int
	done  int
}

func newPrettyProgressHandler(w io.Writer) *prettyProgressHandler {
	return &prettyProgressHandler{w: w, state: &prettyProgressState{}}
}

func (h *prettyProgressHandler) Enabled(context.Context, slog.Level) bool {
	return true
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
		_, err := fmt.Fprintf(
			h.w,
			"\n%sValidating skills: %s%s\n\n%sConfiguration%s\n  %sℹ %d skill%s, %d eval%s%s\n  %sℹ provider=%s target=%s judge=%s baseline=%t%s\n\n%sEvals%s\n",
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
			colorBold,
			colorReset,
		)
		return err
	case "eval_completed":
		h.state.done++
		icon, color := levelIcon(true)
		if attrInt(attrs, "failed") > 0 {
			icon, color = levelIcon(false)
		}
		_, err := fmt.Fprintf(
			h.w,
			"  %s%s [%d/%d] %s / %s:%s %d passed, %d failed, with=%s, baseline=%s, delta=%s\n",
			color,
			icon,
			h.state.done,
			h.state.total,
			attrString(attrs, "rel_path"),
			displayEval(attrs),
			colorReset,
			attrInt(attrs, "passed"),
			attrInt(attrs, "failed"),
			formatRate(attrFloat(attrs, "with_skill_pass_rate")),
			formatOptionalRate(attrs, "without_skill_pass_rate"),
			formatOptionalRate(attrs, "delta"),
		)
		return err
	case "run_completed":
		icon, color := levelIcon(attrBool(attrs, "gate_passed"))
		result := "passed"
		if !attrBool(attrs, "gate_passed") {
			result = fmt.Sprintf("%d failed assertion%s, %d gate failure%s",
				attrInt(attrs, "failed"),
				pluralS(attrInt(attrs, "failed")),
				attrInt(attrs, "gate_failures"),
				pluralS(attrInt(attrs, "gate_failures")),
			)
		}
		_, err := fmt.Fprintf(h.w, "\n%s%sResult: %s %s%s\n", colorBold, color, icon, result, colorReset)
		return err
	default:
		_, err := fmt.Fprintf(h.w, "%s\n", record.Message)
		return err
	}
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
