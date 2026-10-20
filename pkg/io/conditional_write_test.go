/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package io

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
	"gocloud.dev/blob/memblob"
	"gocloud.dev/gcerrors"
)

func TestBlobWriterPreconditions(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		condition *bool
		want      string
	}{
		{"nil overwrites", nil, "replacement"},
		{"false overwrites", new(false), "replacement"},
		{"true preserves", new(true), "original"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bucket := memblob.OpenBucket(nil)
			defer bucket.Close()
			o := &opener{cachedBuckets: map[string]*blob.Bucket{"bucket": bucket}}
			if err := bucket.WriteAll(ctx, "key", []byte("original"), nil); err != nil {
				t.Fatal(err)
			}
			if err := WriteContent(ctx, logrus.NewEntry(logrus.New()), o, "mem://bucket/key", []byte("replacement"), WriterOptions{PreconditionDoesNotExist: tc.condition}); err != nil {
				t.Fatal(err)
			}
			got, err := bucket.ReadAll(ctx, "key")
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBlobConcurrentCreators(t *testing.T) {
	ctx := context.Background()
	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()
	o := &opener{cachedBuckets: map[string]*blob.Bucket{"bucket": bucket}}
	const creators = 8
	writers := make([]WriteCloser, creators)
	// Open all writers before any commits, exposing a check-then-write race.
	for i := range writers {
		var err error
		writers[i], err = o.Writer(ctx, "mem://bucket/key", WriterOptions{PreconditionDoesNotExist: new(true)})
		if err != nil {
			t.Fatal(err)
		}
	}
	results := make([]error, creators)
	var wg sync.WaitGroup
	for i, w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, writeErr := w.Write([]byte(fmt.Sprint(i)))
			closeErr := w.Close()
			results[i] = errors.Join(writeErr, closeErr)
		}()
	}
	wg.Wait()
	winner := -1
	for i, err := range results {
		if err == nil {
			if winner != -1 {
				t.Fatal("multiple creators succeeded")
			}
			winner = i
		} else if !errors.Is(err, gcerrors.ErrFailedPrecondition) {
			t.Fatalf("creator %d: %v", i, err)
		}
	}
	if winner == -1 {
		t.Fatal("no creator succeeded")
	}
	got, err := bucket.ReadAll(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != fmt.Sprint(winner) {
		t.Fatalf("content = %q, winner = %d", got, winner)
	}
}

type failingWriteCloser struct {
	writeErr, closeErr error
	closed             bool
}

func (w *failingWriteCloser) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(p), nil
}
func (w *failingWriteCloser) Close() error { w.closed = true; return w.closeErr }

type writerOpener struct {
	Opener
	writer WriteCloser
}

func (o writerOpener) Writer(context.Context, string, ...WriterOptions) (WriteCloser, error) {
	return o.writer, nil
}

func TestWriteContentPreconditionErrors(t *testing.T) {
	unexpected := errors.New("upload failed")
	precondition := fmt.Errorf("wrapped: %w", gcerrors.ErrFailedPrecondition)
	for _, tc := range []struct {
		name                            string
		writeErr, closeErr, errorWanted error
	}{
		{"write precondition", precondition, nil, nil},
		{"close precondition", nil, precondition, nil},
		{"both preconditions", precondition, precondition, nil},
		{"legacy precondition", PreconditionFailedObjectAlreadyExists, PreconditionFailedObjectAlreadyExists, nil},
		{"unexpected write", unexpected, precondition, unexpected},
		{"unexpected close", precondition, unexpected, unexpected},
		{"other cloud failure", nil, gcerrors.ErrPermissionDenied, gcerrors.ErrPermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &failingWriteCloser{writeErr: tc.writeErr, closeErr: tc.closeErr}
			err := WriteContent(context.Background(), logrus.NewEntry(logrus.New()), writerOpener{writer: w}, "unused", []byte("content"))
			if tc.errorWanted == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.errorWanted != nil && !errors.Is(err, tc.errorWanted) {
				t.Fatalf("error = %v, want %v", err, tc.errorWanted)
			}
			if !w.closed {
				t.Error("writer was not closed")
			}
		})
	}
}
