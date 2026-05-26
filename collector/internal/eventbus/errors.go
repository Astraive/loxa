package eventbus

import "errors"

var (
	// ErrBusClosed is returned when operating on a closed bus.
	ErrBusClosed = errors.New("eventbus: closed")

	// ErrPublishFailed is returned when a publish operation fails.
	ErrPublishFailed = errors.New("eventbus: publish failed")

	// ErrSubscribeFailed is returned when a subscribe operation fails.
	ErrSubscribeFailed = errors.New("eventbus: subscribe failed")

	// ErrUnsupportedType is returned when the configured bus type has no factory.
	ErrUnsupportedType = errors.New("eventbus: unsupported type")

	// ErrTopicEmpty is returned when a topic is not specified.
	ErrTopicEmpty = errors.New("eventbus: topic must not be empty")
)
