package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

func TestWaitForMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		context    func() (context.Context, context.CancelFunc)
		messages   func() <-chan *sarama.ConsumerMessage
		errors     func() <-chan *sarama.ConsumerError
		wantValue  string
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "message arrives",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			messages: func() <-chan *sarama.ConsumerMessage {
				messages := make(chan *sarama.ConsumerMessage, 1)
				messages <- &sarama.ConsumerMessage{Value: []byte("value")}
				return messages
			},
			errors: func() <-chan *sarama.ConsumerError {
				return nil
			},
			wantValue: "value",
		},
		{
			name: "request cancelled",
			context: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			messages: func() <-chan *sarama.ConsumerMessage {
				return make(chan *sarama.ConsumerMessage)
			},
			errors: func() <-chan *sarama.ConsumerError {
				return nil
			},
			wantErr: context.Canceled,
		},
		{
			name: "messages channel closed",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			messages: func() <-chan *sarama.ConsumerMessage {
				messages := make(chan *sarama.ConsumerMessage)
				close(messages)
				return messages
			},
			errors: func() <-chan *sarama.ConsumerError {
				return nil
			},
			wantErrMsg: "messages channel closed",
		},
		{
			name: "consumer error",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			messages: func() <-chan *sarama.ConsumerMessage {
				return make(chan *sarama.ConsumerMessage)
			},
			errors: func() <-chan *sarama.ConsumerError {
				consumerErrors := make(chan *sarama.ConsumerError, 1)
				consumerErrors <- &sarama.ConsumerError{Err: errors.New("consume failed")}
				return consumerErrors
			},
			wantErrMsg: "consume failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := test.context()
			defer cancel()

			message, err := waitForMessage(ctx, test.messages(), test.errors())
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErrMsg != "" && (err == nil || err.Error() != test.wantErrMsg) {
				t.Fatalf("error = %v, want %q", err, test.wantErrMsg)
			}
			if test.wantValue != "" && string(message.Value) != test.wantValue {
				t.Fatalf("message value = %q, want %q", message.Value, test.wantValue)
			}
		})
	}
}

func TestWaitForMessageTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := waitForMessage(ctx, make(chan *sarama.ConsumerMessage), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestRemainingTimeoutCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	_, err := remainingTimeout(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
}
