package valkey

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestCheckIncludesCleanupInOutcome(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		getErr  error
		delErr  error
		wantErr string
	}{
		{name: "success"},
		{name: "read failure still deletes key", getErr: errors.New("read unavailable"), wantErr: "read unavailable"},
		{name: "delete failure fails check", delErr: errors.New("delete denied"), wantErr: "delete denied"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hook := &commandHook{getErr: tt.getErr, delErr: tt.delErr}
			client := redis.NewClient(&redis.Options{MaxRetries: -1})
			client.AddHook(hook)
			defer func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			}()
			err := check(t.Context(), client)
			if tt.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if !hook.deleted {
				t.Fatal("probe key was not cleaned up")
			}
		})
	}
}

func TestCheckRequiresTLS(t *testing.T) {
	t.Setenv("VALKEY_URI_CONTESTS", "valkey://localhost:6379")
	if err := Check(t.Context()); err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("error = %v, want TLS requirement", err)
	}
}

type commandHook struct {
	value   string
	getErr  error
	delErr  error
	deleted bool
}

func (*commandHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *commandHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "set":
			h.value = cmd.Args()[2].(string)
			cmd.(*redis.StatusCmd).SetVal("OK")
		case "get":
			if h.getErr != nil {
				return h.getErr
			}
			cmd.(*redis.StringCmd).SetVal(h.value)
		case "del":
			h.deleted = true
			if h.delErr != nil {
				return h.delErr
			}
			cmd.(*redis.IntCmd).SetVal(1)
		default:
			return errors.New("unexpected command")
		}
		return nil
	}
}

func (*commandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
