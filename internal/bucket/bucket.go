// Package bucket checks object storage by writing, reading and deleting an object.
package bucket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/nais/contests/internal/uniqid"
)

const payload = "contests"

// Check verifies bucket access using the workload identity.
func Check(ctx context.Context, name string) (err error) {
	if name == "" {
		return errors.New("BUCKET_NAME is required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("create storage client: %w", err)
	}
	defer func() { err = errors.Join(err, client.Close()) }()
	return check(ctx, client.Bucket(name))
}

func check(ctx context.Context, bucket *storage.BucketHandle) (err error) {
	suffix, err := uniqid.Suffix()
	if err != nil {
		return err
	}
	obj := bucket.Object("contests-" + suffix)
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if cleanupErr := obj.Delete(cleanupCtx); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("delete object: %w", cleanupErr))
		}
	}()

	writer := obj.NewWriter(ctx)
	_, writeErr := io.WriteString(writer, payload)
	if err := errors.Join(writeErr, writer.Close()); err != nil {
		return fmt.Errorf("write object: %w", err)
	}
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open object: %w", err)
	}
	value, readErr := io.ReadAll(io.LimitReader(reader, int64(len(payload)+1)))
	if err := errors.Join(readErr, reader.Close()); err != nil {
		return fmt.Errorf("read object: %w", err)
	}
	if string(value) != payload {
		return errors.New("object content did not match")
	}
	return nil
}
