/*
Copyright 2018 The Kubernetes Authors.

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

// Package reporter contains helpers for publishing statues to Pub
// statuses in GitHub.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prowapi "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"
	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/crier/reporters/criercommonlib"
	"sigs.k8s.io/prow/pkg/io/providers"
	"sigs.k8s.io/prow/pkg/spyglass/api"
)

const (
	// PubSubProjectLabel annotation
	PubSubProjectLabel = "prow.k8s.io/pubsub.project"
	// PubSubTopicLabel annotation
	PubSubTopicLabel = "prow.k8s.io/pubsub.topic"
	// PubSubRunIDLabel annotation
	PubSubRunIDLabel = "prow.k8s.io/pubsub.runID"
)

// ReportMessage is a message structure used to pass a prowjob status to Pub/Sub topic.s
type ReportMessage struct {
	Project string               `json:"project"`
	Topic   string               `json:"topic"`
	RunID   string               `json:"runid"`
	Status  prowapi.ProwJobState `json:"status"`
	URL     string               `json:"url"`
	GCSPath string               `json:"gcs_path"`
	Refs    []prowapi.Refs       `json:"refs,omitempty"`
	JobType prowapi.ProwJobType  `json:"job_type"`
	JobName string               `json:"job_name"`
	Message string               `json:"message,omitempty"`
}

// Client is a reporter client fed to crier controller
type Client struct {
	config config.Getter
	// newPubSubClient creates the Pub/Sub client used to report to the given
	// project. Tests override it to point at a fake server.
	newPubSubClient func(ctx context.Context, project string) (*pubsub.Client, error)

	// clients and publishers are cached per project/topic and reused for the
	// lifetime of the process. Crier does not offer a shutdown hook where they
	// could be closed, so we intentionally never call Close/Stop on them.
	mu         sync.Mutex
	clients    map[string]*pubsub.Client
	publishers map[publisherKey]*pubsub.Publisher
}

type publisherKey struct {
	project string
	topic   string
}

// NewReporter creates a new Pub/Sub reporter
func NewReporter(cfg config.Getter) *Client {
	return &Client{
		config: cfg,
		newPubSubClient: func(ctx context.Context, project string) (*pubsub.Client, error) {
			return pubsub.NewClient(ctx, project)
		},
	}
}

// GetName returns the name of the reporter
func (c *Client) GetName() string {
	return "pubsub-reporter"
}

// getClient returns the cached client for project, creating it on first use.
// The returned client must not be closed by the caller.
func (c *Client) getClient(ctx context.Context, project string) (*pubsub.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok := c.clients[project]; ok {
		return client, nil
	}

	client, err := c.newPubSubClient(ctx, project)
	if err != nil {
		return nil, err
	}
	if c.clients == nil {
		c.clients = make(map[string]*pubsub.Client)
	}
	c.clients[project] = client
	return client, nil
}

// getPublisher returns the cached publisher for project/topic, creating it on
// first use. The returned publisher must not be stopped by the caller.
func (c *Client) getPublisher(client *pubsub.Client, project, topic string) *pubsub.Publisher {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := publisherKey{project: project, topic: topic}
	if publisher, ok := c.publishers[key]; ok {
		return publisher
	}

	publisher := client.Publisher(topic)
	if c.publishers == nil {
		c.publishers = make(map[publisherKey]*pubsub.Publisher)
	}
	c.publishers[key] = publisher
	return publisher
}

// resetCache drops all cached clients and publishers. It exists so tests can
// isolate reporter instances; production code keeps them for the process
// lifetime.
func (c *Client) resetCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients = nil
	c.publishers = nil
}

func findLabels(pj *prowapi.ProwJob, labels ...string) map[string]string {
	// Support checking for both labels(deprecated) and annotations(new) for backward compatibility
	pubSubMap := map[string]string{}
	for _, label := range labels {
		if pj.Annotations[label] != "" {
			pubSubMap[label] = pj.Annotations[label]
		} else {
			pubSubMap[label] = pj.Labels[label]
		}
	}
	return pubSubMap
}

// ShouldReport tells if a prowjob should be reported by this reporter
func (c *Client) ShouldReport(_ context.Context, _ *logrus.Entry, pj *prowapi.ProwJob) bool {
	pubSubMap := findLabels(pj, PubSubProjectLabel, PubSubTopicLabel)
	return pubSubMap[PubSubProjectLabel] != "" && pubSubMap[PubSubTopicLabel] != ""
}

// Report takes a prowjob, and generate a pubsub ReportMessage and publish to specific Pub/Sub topic
// based on Pub/Sub related labels if they exist in this prowjob
func (c *Client) Report(ctx context.Context, l *logrus.Entry, pj *prowapi.ProwJob) ([]*prowapi.ProwJob, *reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	message := c.generateMessageFromPJ(pj)
	client, err := c.getClient(ctx, message.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create pubsub Client: %w", err)
	}

	l = l.WithFields(logrus.Fields{"project": message.Project, "topic": message.Topic, "run-id": message.RunID, "status": pj.Status.State})
	l.Debug("Reporting prowjob status to pubsub.")

	d, err := json.Marshal(message)
	if err != nil {
		l.WithError(err).Debug("Failed marshalling pubsub message.")
		return nil, nil, fmt.Errorf("could not marshal pubsub report: %w", err)
	}

	publisher := c.getPublisher(client, message.Project, message.Topic)
	if _, err := publisher.Publish(ctx, &pubsub.Message{Data: d}).Get(ctx); err != nil {
		wrappedError := fmt.Errorf(
			"failed to publish pubsub message with run ID %q to topic: \"%s/%s\". %v",
			message.RunID, message.Project, message.Topic, err)

		// It would be a user error if the topic doesn't exist, return a user
		// error in this case so that we can avoid logging on error level.
		// Use a fresh context: the publish may have exhausted the report one.
		getCtx, getCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer getCancel()
		_, getErr := client.TopicAdminClient.GetTopic(getCtx, &pubsubpb.GetTopicRequest{Topic: publisher.String()})
		if status.Code(getErr) == codes.NotFound {
			l.Debug("Pubsub topic doesn't exist.")
			return nil, nil, criercommonlib.UserError(wrappedError)
		}

		l.WithError(err).Debug("Failed sending pubsub message.")
		return nil, nil, wrappedError
	}

	return []*prowapi.ProwJob{pj}, nil, nil
}

func (c *Client) generateMessageFromPJ(pj *prowapi.ProwJob) *ReportMessage {
	pubSubMap := findLabels(pj, PubSubProjectLabel, PubSubTopicLabel, PubSubRunIDLabel)
	var refs []prowapi.Refs
	if pj.Spec.Refs != nil {
		refs = append(refs, *pj.Spec.Refs)
	}
	refs = append(refs, pj.Spec.ExtraRefs...)

	var storagePath string
	// calculate storagePath if pj.Status.URL is set
	if pj.Status.URL != "" {
		// example:
		// * pj.Status.URL: https://prow.k8s.io/view/gs/kubernetes-jenkins/logs/ci-benchmark-microbenchmarks/1258197944759226371
		// * prefix: https://prow.k8s.io/view/
		// * storageURLPath: gs/kubernetes-jenkins/logs/ci-benchmark-microbenchmarks/1258197944759226371
		prefix := c.config().Plank.GetJobURLPrefix(pj)

		storageURLPath := strings.TrimPrefix(pj.Status.URL, prefix)
		if strings.HasPrefix(storageURLPath, api.GCSKeyType) {
			storageURLPath = strings.Replace(storageURLPath, api.GCSKeyType, providers.GS, 1)
		}

		if providers.HasStorageProviderPrefix(storageURLPath) {
			storagePathSegments := strings.SplitN(storageURLPath, "/", 2)
			if len(storagePathSegments) == 1 {
				storagePath = storagePathSegments[0]
			} else {
				storagePath = fmt.Sprintf("%s://%s", storagePathSegments[0], storagePathSegments[1])
			}
		} else {
			storagePath = fmt.Sprintf("%s://%s", providers.GS, storageURLPath)
		}

	}

	return &ReportMessage{
		Project: pubSubMap[PubSubProjectLabel],
		Topic:   pubSubMap[PubSubTopicLabel],
		RunID:   pubSubMap[PubSubRunIDLabel],
		Status:  pj.Status.State,
		URL:     pj.Status.URL,
		GCSPath: storagePath,
		Refs:    refs,
		JobType: pj.Spec.Type,
		JobName: pj.Spec.Job,
		Message: pj.Status.Description,
	}
}
