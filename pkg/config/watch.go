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

package config

import (
	"context"
	"errors"
	"reflect"

	"github.com/sirupsen/logrus"
)

// ErrWatchDone is returned by WatchConfig when the extra done channel fires.
var ErrWatchDone = errors.New("config watch done")

// WatchConfig watches agent for config changes and calls onChange whenever the
// value returned by sections for the latest config differs from the value for
// the previously observed config.
//
// Only the latest config is considered, since the agent coalesces undrained
// deltas. onChange runs before the new snapshot is recorded, so callers can
// apply the config first. An error returned by onChange is returned by
// WatchConfig.
//
// WatchConfig returns nil when ctx is cancelled. If extraDone is non-nil, it is
// called before each select to obtain a channel whose closure stops the watch
// with ErrWatchDone. Calling it repeatedly lets callers replace the channel
// after restarting their workers.
func WatchConfig[T any](ctx context.Context, agent *Agent, extraDone func() <-chan struct{}, sections func(*Config) T, onChange func(*Config) error) error {
	subscription := agent.Subscribe()
	current := sections(agent.Config())

	for {
		var done <-chan struct{}
		if extraDone != nil {
			done = extraDone()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return ErrWatchDone
		case delta := <-subscription:
			logrus.Info("Received new config")
			next := sections(&delta.After)
			if !reflect.DeepEqual(current, next) {
				if err := onChange(&delta.After); err != nil {
					return err
				}
				current = next
			}
		}
	}
}
