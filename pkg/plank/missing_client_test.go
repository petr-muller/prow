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

package plank

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clocktesting "k8s.io/utils/clock/testing"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	prowv1 "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"
	"sigs.k8s.io/prow/pkg/testutil"
)

func TestMissingBuildClientRecovery(t *testing.T) {
	for _, state := range []prowv1.ProwJobState{prowv1.TriggeredState, prowv1.PendingState, prowv1.AbortedState} {
		for _, outcome := range []string{"recover", "recover missing pod", "recover after status patch failure", "nil client", "expire", "unknown", "disabled", "disable while waiting", "invalid timestamp", "future timestamp"} {
			t.Run(string(state)+"/"+outcome, func(t *testing.T) {
				ctx := context.Background()
				cfg := newFakeConfigAgent(t, 0, nil)
				pj := &prowv1.ProwJob{
					ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "prowjobs"},
					Spec:       prowv1.ProwJobSpec{Agent: prowv1.KubernetesAgent, Type: prowv1.PeriodicJob, Job: "test-job", Cluster: "build", PodSpec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}}},
					Status:     prowv1.ProwJobStatus{State: state},
				}
				mgr, err := testutil.NewFakeManager(ctx, []runtime.Object{pj}, func(ctx context.Context, indexer ctrlruntimeclient.FieldIndexer) error {
					return setupIndexes(ctx, indexer, cfg.Config)
				})
				if err != nil {
					t.Fatal(err)
				}
				now := time.Date(2026, 10, 10, 17, 0, 0, 0, time.UTC)
				fc := clocktesting.NewFakeClock(now)
				buildIDs := 0
				tot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { buildIDs++; handleTot(w, req) }))
				defer tot.Close()
				makeReconciler := func() *reconciler {
					r := newReconciler(ctx, mgr.GetClient(), nil, cfg.Config, nil, tot.URL)
					r.clock = fc
					r.configuredClusters = sets.New("build") // Includes aliases filtered out at startup.
					return r
				}
				r := makeReconciler()
				if outcome == "nil client" {
					r.buildClients = map[string]buildClient{"build": {}}
				}
				if outcome == "unknown" {
					r.configuredClusters = nil
				}
				if outcome == "disabled" {
					cfg.c.DisabledClusters = []string{"build"}
				}
				if outcome == "invalid timestamp" || outcome == "future timestamp" {
					stamp := "invalid"
					if outcome == "future timestamp" {
						stamp = now.Add(time.Hour).Format(time.RFC3339Nano)
					}
					pj.Annotations = map[string]string{missingBuildClientSinceAnnotation: stamp}
					if err := mgr.GetClient().Update(ctx, pj); err != nil {
						t.Fatal(err)
					}
				}
				request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: pj.Namespace, Name: pj.Name}}
				readJob := func() *prowv1.ProwJob {
					t.Helper()
					got := &prowv1.ProwJob{}
					if err := mgr.GetClient().Get(ctx, request.NamespacedName, got); err != nil {
						t.Fatal(err)
					}
					return got
				}
				run := func() reconcile.Result {
					t.Helper()
					result, err := r.Reconcile(ctx, request)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				result := run()
				if outcome == "unknown" || outcome == "disabled" || outcome == "invalid timestamp" || outcome == "future timestamp" {
					got := readJob()
					if !got.Complete() || got.Status.State != prowv1.ErrorState || result.RequeueAfter != 0 {
						t.Fatalf("expected terminal error, got %+v, %+v", got.Status, result)
					}
					if outcome == "unknown" || outcome == "disabled" {
						if !strings.Contains(got.Status.Description, outcome) {
							t.Fatalf("unclear terminal description: %s", got.Status.Description)
						}
						if got.Annotations[missingBuildClientSinceAnnotation] != "" {
							t.Fatal("terminal aliases received a recovery budget")
						}
					}
					return
				}
				stamp := readJob().Annotations[missingBuildClientSinceAnnotation]
				if stamp != now.Format(time.RFC3339Nano) {
					t.Fatalf("first observation not persisted: %s", stamp)
				}
				// Simulate PCM restart; the persisted deadline must survive a new reconciler.
				r = makeReconciler()
				for i := 0; i < 3; i++ {
					if result.RequeueAfter != missingBuildClientRetryInterval {
						t.Fatalf("unexpected retry: %+v", result)
					}
					got := readJob()
					if got.Complete() || got.Status.State != state || got.Status.BuildID != "" {
						t.Fatalf("job changed while waiting: %+v", got.Status)
					}
					fc.Step(result.RequeueAfter)
					result = run()
					if readJob().Annotations[missingBuildClientSinceAnnotation] != stamp {
						t.Fatal("retry renewed recovery window")
					}
				}
				if buildIDs != 0 {
					t.Fatalf("allocated %d build IDs while client absent", buildIDs)
				}
				if outcome == "disable while waiting" {
					cfg.c.DisabledClusters = []string{"build"}
					result = run()
					if got := readJob(); !got.Complete() || !strings.Contains(got.Status.Description, "disabled") || result.RequeueAfter != 0 {
						t.Fatalf("disable did not terminate recovery: %+v, %+v", got.Status, result)
					}
					return
				}
				if outcome == "expire" {
					// Check the last retry is capped at the remaining budget.
					fc.SetTime(now.Add(missingBuildClientRecoveryWindow - time.Second))
					if result = run(); result.RequeueAfter != time.Second {
						t.Fatalf("retry exceeds deadline: %+v", result)
					}
					fc.Step(time.Second)
					result = run()
					got := readJob()
					if !got.Complete() || got.Status.State != prowv1.ErrorState || !strings.Contains(got.Status.Description, "5m0s recovery window expired") || result.RequeueAfter != 0 {
						t.Fatalf("expected expiry, got %+v, %+v", got.Status, result)
					}
					return
				}
				// Restart with a usable client. Pending/aborted jobs already have a pod.
				builder := fake.NewClientBuilder()
				if state != prowv1.TriggeredState && outcome != "recover missing pod" {
					builder.WithObjects(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pj.Name, Namespace: "pods"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}})
				}
				pods := &countPodCreates{Client: builder.Build()}
				r = makeReconciler()
				r.buildClients = map[string]buildClient{"build": {Client: pods}}
				if outcome == "recover after status patch failure" && state == prowv1.TriggeredState {
					r.pjClient = &failOnePatch{Client: mgr.GetClient()}
					if _, err := r.Reconcile(ctx, request); err == nil {
						t.Fatal("expected injected status patch failure")
					}
					if got := readJob(); got.Status.State != state || got.Complete() {
						t.Fatalf("status changed despite failed patch: %+v", got.Status)
					}
				}
				run()
				run() // Repeated reconciliation must reuse the pod, not allocate/create again.
				got := readJob()
				wantState := prowv1.PendingState
				wantCreates := 0
				if state == prowv1.TriggeredState || (state == prowv1.PendingState && outcome == "recover missing pod") {
					wantCreates = 1
				}
				if state == prowv1.AbortedState {
					wantState = state
				}
				if got.Status.State != wantState || got.Complete() != (state == prowv1.AbortedState) || got.Annotations[missingBuildClientSinceAnnotation] != "" {
					t.Fatalf("did not recover: %+v", got)
				}
				if pods.creates != wantCreates || buildIDs != wantCreates {
					t.Fatalf("pod creates=%d, build IDs=%d, want %d", pods.creates, buildIDs, wantCreates)
				}
				list := &corev1.PodList{}
				if err := pods.List(ctx, list); err != nil {
					t.Fatal(err)
				}
				wantPods := 1
				if state == prowv1.AbortedState {
					wantPods = 0
				}
				if len(list.Items) != wantPods {
					t.Fatalf("pods=%d, want %d", len(list.Items), wantPods)
				}
			})
		}
	}
}

type countPodCreates struct {
	ctrlruntimeclient.Client
	creates int
}

func (c *countPodCreates) Create(ctx context.Context, obj ctrlruntimeclient.Object, opts ...ctrlruntimeclient.CreateOption) error {
	c.creates++
	return c.Client.Create(ctx, obj, opts...)
}

func TestMissingBuildClientOperations(t *testing.T) {
	cfg := newFakeConfigAgent(t, 0, nil)
	r := newReconciler(context.Background(), nil, nil, cfg.Config, nil, "http://must-not-request-build-id")
	r.configuredClusters = sets.New("default")
	pj := &prowv1.ProwJob{} // Empty cluster uses the default alias.
	for _, operation := range []string{"pod", "delete", "abort", "start"} {
		t.Run(operation, func(t *testing.T) {
			var err error
			switch operation {
			case "pod":
				_, _, err = r.pod(context.Background(), pj)
			case "delete":
				err = r.deletePod(context.Background(), pj)
			case "abort":
				err = r.syncAbortedJob(context.Background(), pj)
			case "start":
				_, _, err = r.startPod(context.Background(), pj)
			}
			var missing *missingBuildClientError
			if !errors.As(err, &missing) {
				t.Fatalf("expected recoverable absence, got %v", err)
			}
			if pj.Status.BuildID != "" {
				t.Fatal("allocated build ID")
			}
		})
	}
}

// A created pod can precede its ProwJob status update, especially during restart.
type failOnePatch struct {
	ctrlruntimeclient.Client
	failed bool
}

func (c *failOnePatch) Patch(ctx context.Context, obj ctrlruntimeclient.Object, patch ctrlruntimeclient.Patch, opts ...ctrlruntimeclient.PatchOption) error {
	if !c.failed {
		c.failed = true
		return errors.New("injected patch failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
