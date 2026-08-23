package jsonenc

// Writer is a low-level append-only JSON byte writer.
// It tracks object/array nesting to insert commas automatically.
type Writer struct {
	buf   []byte
	first []bool // stack: true = first field in current object/array
}

// New returns a Writer that appends to buf.
func New(buf []byte) *Writer {
	return &Writer{buf: buf}
}

// Bytes returns the accumulated buffer.
func (w *Writer) Bytes() []byte { return w.buf }

// Reset resets the writer to an empty state, reusing the underlying buffer.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
	w.first = w.first[:0]
}

// ── Object / array delimiters ─────────────────────────────────────────────────

// BeginObject writes '{'.
func (w *Writer) BeginObject() {
	w.buf = append(w.buf, '{')
	w.first = append(w.first, true)
}

// EndObject writes '}'.
func (w *Writer) EndObject() {
	w.buf = append(w.buf, '}')
	if n := len(w.first); n > 0 {
		w.first = w.first[:n-1]
	}
}

// BeginArray writes '['.
func (w *Writer) BeginArray() {
	w.buf = append(w.buf, '[')
	w.first = append(w.first, true)
}

// EndArray writes ']'.
func (w *Writer) EndArray() {
	w.buf = append(w.buf, ']')
	if n := len(w.first); n > 0 {
		w.first = w.first[:n-1]
	}
}

// sep inserts a ',' separator between elements.
func (w *Writer) sep() {
	n := len(w.first)
	if n == 0 {
		return
	}
	if w.first[n-1] {
		w.first[n-1] = false
	} else {
		w.buf = append(w.buf, ',')
	}
}

// ── Key / value helpers ───────────────────────────────────────────────────────

// AppendKey writes a comma-separated JSON key followed by ':'.
func (w *Writer) AppendKey(key string) {
	w.sep()
	w.buf = AppendEscapedString(w.buf, key)
	w.buf = append(w.buf, ':')
}

// AppendStringField writes key:"value".
func (w *Writer) AppendStringField(key, val string) {
	w.AppendKey(key)
	w.buf = AppendEscapedString(w.buf, val)
}

// AppendInt64Field writes key:number (int64).
func (w *Writer) AppendInt64Field(key string, val int64) {
	w.AppendKey(key)
	w.buf = AppendInt64(w.buf, val)
}

// AppendUint64Field writes key:number (uint64).
func (w *Writer) AppendUint64Field(key string, val uint64) {
	w.AppendKey(key)
	w.buf = AppendUint64(w.buf, val)
}

// AppendFloat64Field writes key:number (float64).
func (w *Writer) AppendFloat64Field(key string, val float64) {
	w.AppendKey(key)
	w.buf = AppendFloat64(w.buf, val)
}

// AppendBoolField writes key:true|false.
func (w *Writer) AppendBoolField(key string, val bool) {
	w.AppendKey(key)
	if val {
		w.buf = append(w.buf, "true"...)
	} else {
		w.buf = append(w.buf, "false"...)
	}
}

// AppendNullField writes key:null.
func (w *Writer) AppendNullField(key string) {
	w.AppendKey(key)
	w.buf = append(w.buf, "null"...)
}

// AppendRaw appends rawJSON bytes directly (no key, no separator).
func (w *Writer) AppendRaw(rawJSON []byte) {
	w.buf = append(w.buf, rawJSON...)
}

// AppendString writes a bare JSON-escaped string (with quotes, no key).
func (w *Writer) AppendString(s string) {
	w.sep()
	w.buf = AppendEscapedString(w.buf, s)
}

// Newline appends a newline byte (for NDJSON).
func (w *Writer) Newline() {
	w.buf = append(w.buf, '\n')
}
