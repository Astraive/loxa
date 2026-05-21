package ingest

const (
	StatusAccepted = "accepted"
	StatusPartial  = "partial"
	StatusRejected = "rejected"
	StatusInvalid  = "invalid"
)

type EventAck struct {
	Index     int    `json:"index,omitempty"`
	EventID   string `json:"event_id,omitempty"`
	Status    string `json:"status"`
	Retryable bool   `json:"retryable"`
	Reason    string `json:"reason"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
}

type EventError struct {
	Index     int    `json:"index"`
	EventID   string `json:"event_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Response struct {
	RequestID    string       `json:"request_id,omitempty"`
	Status       string       `json:"status,omitempty"`
	Accepted     int          `json:"accepted"`
	Rejected     int          `json:"rejected"`
	Duplicates   int          `json:"duplicates,omitempty"`
	Deduped      int          `json:"deduped,omitempty"`
	Invalid      int          `json:"invalid"`
	Errors       []EventError `json:"errors,omitempty"`
	Acks         []EventAck   `json:"acks,omitempty"`
	RetryAfterMS int          `json:"retry_after_ms,omitempty"`
	Reason       string       `json:"reason,omitempty"`
	Error        string       `json:"error,omitempty"`
}

func (r *Response) AddAccepted(index int, eventID string) {
	r.Accepted++
	r.Acks = append(r.Acks, EventAck{
		Index:     index,
		EventID:   eventID,
		Status:    StatusAccepted,
		Retryable: false,
		Reason:    StatusAccepted,
		Code:      StatusAccepted,
		Message:   "event accepted",
	})
}

func (r *Response) AddDuplicate(index int, eventID string) {
	r.Accepted++
	r.Duplicates++
	r.Deduped++
	r.Acks = append(r.Acks, EventAck{
		Index:     index,
		EventID:   eventID,
		Status:    StatusAccepted,
		Retryable: false,
		Reason:    "duplicate_event_id",
		Code:      "duplicate_event_id",
		Message:   "duplicate event accepted by idempotency",
		Duplicate: true,
	})
}

func (r *Response) AddInvalid(index int, eventID, code, message string) {
	r.Invalid++
	r.addError(index, eventID, code, message, false, StatusInvalid)
}

func (r *Response) AddRejected(index int, eventID, code, message string, retryable bool) {
	r.Rejected++
	r.addError(index, eventID, code, message, retryable, StatusRejected)
}

func (r *Response) addError(index int, eventID, code, message string, retryable bool, status string) {
	if code == "" {
		code = status
	}
	if message == "" {
		message = code
	}
	r.Errors = append(r.Errors, EventError{
		Index:     index,
		EventID:   eventID,
		Code:      code,
		Message:   message,
		Retryable: retryable,
	})
	r.Acks = append(r.Acks, EventAck{
		Index:     index,
		EventID:   eventID,
		Status:    status,
		Retryable: retryable,
		Reason:    code,
		Code:      code,
		Message:   message,
	})
}

func (r *Response) Finalize() {
	switch {
	case r.Accepted > 0 && (r.Invalid > 0 || r.Rejected > 0):
		r.Status = StatusPartial
	case r.Accepted > 0:
		r.Status = StatusAccepted
	case r.Invalid > 0:
		r.Status = StatusInvalid
	default:
		r.Status = StatusRejected
	}
}
