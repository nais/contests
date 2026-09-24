package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestWaitForMessageCancelledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		_, err := waitForMessage(ctx, make(chan *sarama.ConsumerMessage), nil)
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want cancelled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("message wait did not stop on cancellation")
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

func TestRemainingTimeoutNeedsUsableDeadline(t *testing.T) {
	if _, err := remainingTimeout(context.Background()); err == nil {
		t.Fatal("missing deadline was accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	if _, err := remainingTimeout(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestKafkaProbeTimeout(t *testing.T) {
	if probeTimeout != 8*time.Second {
		t.Fatalf("probe timeout = %s, want 8s", probeTimeout)
	}
}

func TestExpiredRequestReturnsWithoutConnecting(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/kafka", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	start := time.Now()

	(&Kafka{brokers: []string{"invalid:9092"}}).Handler()(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("expired probe took %s", elapsed)
	}
}

func TestRunProbeReturnsOnDeadlineAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	release := make(chan struct{})
	closed := make(chan struct{})
	start := time.Now()

	err := runProbe(ctx, func(context.Context) error {
		defer close(closed)
		<-release
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("blocked operation kept probe alive for %s", elapsed)
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("probe worker did not clean up")
	}
}

func TestBrokerConnectionClosesAfterDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	closed := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			close(closed)
			return
		}
		defer func() { _ = conn.Close() }()
		var buffer [1]byte
		for {
			if _, err := conn.Read(buffer[:]); err != nil {
				close(closed)
				return
			}
		}
	}()

	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{MinVersion: tls.VersionTLS12}
	config.Version = sarama.V0_10_2_0
	k := &Kafka{config: config, brokers: []string{listener.Addr().String()}}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	recorder := httptest.NewRecorder()
	start := time.Now()

	k.Handler()(recorder, httptest.NewRequest(http.MethodGet, "/kafka", nil).WithContext(ctx))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Fatalf("stalled broker kept handler alive for %s", elapsed)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("broker connection was not closed after timeout")
	}
}

func TestOperationTimeoutConfig(t *testing.T) {
	base := sarama.NewConfig()
	base.Version = sarama.V0_10_2_0
	base.Net.TLS.Enable = true
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	timeout, err := remainingTimeout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if timeout <= 0 || timeout > 100*time.Millisecond {
		t.Fatalf("remaining timeout = %s, want within 100ms", timeout)
	}
	config := withOperationTimeout(base, timeout)
	for name, operationTimeout := range map[string]time.Duration{
		"dial": config.Net.DialTimeout, "read": config.Net.ReadTimeout,
		"write": config.Net.WriteTimeout, "producer": config.Producer.Timeout,
		"metadata": config.Metadata.Timeout,
	} {
		if operationTimeout != timeout {
			t.Errorf("%s timeout = %s, want remaining deadline %s", name, operationTimeout, timeout)
		}
	}
	if config.Metadata.Retry.Max != 0 || config.Metadata.Retry.Backoff != 0 ||
		config.Producer.Retry.Max != 0 || config.Producer.Retry.Backoff != 0 ||
		config.Consumer.Retry.Max != 1 || config.Consumer.Retry.Backoff != 0 {
		t.Fatal("probe config retained retry delays")
	}
	if config.Version != base.Version || !config.Net.TLS.Enable {
		t.Fatal("probe config changed Kafka version or TLS")
	}
	if base.Metadata.Retry.Max == 0 || base.Metadata.Timeout != 0 {
		t.Fatal("probe config changed base config")
	}
}
