package skilpel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

func progressLogger(w io.Writer, format string) *slog.Logger {
	switch format {
	case "pretty":
		return slog.New(newPrettyProgressHandler(w))
	case "auto":
		if file, ok := w.(*os.File); ok && isTerminal(file) {
			return slog.New(newPrettyProgressHandler(w))
		}
	}
	return structuredLogger(w)
}

func structuredLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
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
	}))
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

type prettyProgressHandler struct {
	w     io.Writer
	mu    sync.Mutex
	total int
	done  int
}

func newPrettyProgressHandler(w io.Writer) *prettyProgressHandler {
	return &prettyProgressHandler{w: w}
}

func (h *prettyProgressHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *prettyProgressHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	switch attrs["event"] {
	case "run_started":
		h.total = attrInt(attrs, "evals")
		h.done = 0
		_, err := fmt.Fprintf(
			h.w,
			"skilpel: %d skills, %d evals | provider=%s target=%s judge=%s baseline=%t\n",
			attrInt(attrs, "skills"),
			h.total,
			attrString(attrs, "provider"),
			attrString(attrs, "target"),
			attrString(attrs, "judge"),
			attrBool(attrs, "baseline"),
		)
		return err
	case "eval_completed":
		h.done++
		status := "PASS"
		if attrInt(attrs, "failed") > 0 {
			status = "FAIL"
		}
		_, err := fmt.Fprintf(
			h.w,
			"%s [%d/%d] %s | %s | assertions=%d/%d | with=%s baseline=%s delta=%s\n",
			status,
			h.done,
			h.total,
			attrString(attrs, "rel_path"),
			displayEval(attrs),
			attrInt(attrs, "passed"),
			attrInt(attrs, "total"),
			formatRate(attrFloat(attrs, "with_skill_pass_rate")),
			formatOptionalRate(attrs, "without_skill_pass_rate"),
			formatOptionalRate(attrs, "delta"),
		)
		return err
	default:
		_, err := fmt.Fprintf(h.w, "%s\n", record.Message)
		return err
	}
}

func (h *prettyProgressHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *prettyProgressHandler) WithGroup(string) slog.Handler {
	return h
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
