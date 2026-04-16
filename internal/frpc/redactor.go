package frpc

import (
	"bytes"
	"io"
	"strings"
	"sync"

	"frp-helper/internal/model"
)

type RedactingWriter struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	recent  []string
	secrets []string
	writers []io.Writer
}

func NewRedactingWriter(writers []io.Writer, secrets []string) *RedactingWriter {
	return &RedactingWriter{
		secrets: secrets,
		writers: writers,
	}
}

func (w *RedactingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, b := range p {
		w.buffer.WriteByte(b)
		if b == '\n' {
			if err := w.flushLocked(); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

func (w *RedactingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *RedactingWriter) RecentText() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.recent, "\n")
}

func (w *RedactingWriter) flushLocked() error {
	if w.buffer.Len() == 0 {
		return nil
	}
	line := model.Sanitize(w.buffer.String(), w.secrets)
	w.buffer.Reset()
	for _, sink := range w.writers {
		if _, err := io.WriteString(sink, line); err != nil {
			return err
		}
	}
	trimmed := strings.TrimSpace(line)
	if trimmed != "" {
		w.recent = append(w.recent, trimmed)
		if len(w.recent) > 50 {
			w.recent = append([]string(nil), w.recent[len(w.recent)-50:]...)
		}
	}
	return nil
}
