/*
Copyright 2026 The BlanketOps Authors.
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

package grpc_test

// githubEventServer is a real GitHubEventServiceServer implementation,
// following the same pattern as buildServer/deploymentServer/
// packageServer/serviceUnitServer — see build_server_test.go's doc
// comment for the rationale.
//
// GitHubEvent CRs conventionally live in the argo-events namespace in
// production (see the stub's own comment), but this server uses the
// same shared testNamespace ("default") as every other service here —
// nothing in this suite runs the real controller that would reconcile
// the owned EventSource/EventBus/Sensor into argo-events, so there's no
// real behavior tied to that namespace to exercise.
//
// Unlike the CRUD services wired so far, GitHubEventService has no
// Update/Patch RPC — events are immutable once accepted — and
// WatchGitHubEvent accepts either a specific name or a label selector
// (mutually exclusive), the latter a real server-side watch filter via
// client.MatchingLabelsSelector, not client-side filtering.

import (
	"context"
	"sort"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	githubeventpb "github.com/blanketops/environments-contract/blanketops/events/v1alpha1"
)

type githubEventServer struct {
	githubeventpb.UnimplementedGitHubEventServiceServer

	client    client.WithWatch
	namespace string
}

func githubEventSpecFromRaw(raw runtime.RawExtension) (*githubeventpb.GitHubEventSpec, error) {
	spec := &githubeventpb.GitHubEventSpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal github event spec: %v", err)
	}
	return spec, nil
}

func githubEventSpecToRaw(spec *githubeventpb.GitHubEventSpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal github event spec: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func githubEventStatusFromRaw(raw runtime.RawExtension) (*githubeventpb.GitHubEventStatus, error) {
	st := &githubeventpb.GitHubEventStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal github event status: %v", err)
	}
	return st, nil
}

func githubEventToProto(cr *eventsv1alpha1.GitHubEvent) (*githubeventpb.GitHubEvent, error) {
	spec, err := githubEventSpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := githubEventStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &githubeventpb.GitHubEvent{
		Metadata: &commonv1.Metadata{
			Id:         string(cr.UID),
			Name:       cr.Name,
			Labels:     cr.Labels,
			Generation: cr.Generation,
		},
		Spec:   spec,
		Status: st,
	}, nil
}

func (s *githubEventServer) get(ctx context.Context, name string) (*eventsv1alpha1.GitHubEvent, error) {
	cr := &eventsv1alpha1.GitHubEvent{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "github event %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get github event %q: %v", name, err)
	}
	return cr, nil
}

func (s *githubEventServer) CreateGitHubEvent(
	ctx context.Context, req *githubeventpb.CreateGitHubEventRequest,
) (*githubeventpb.CreateGitHubEventResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := githubEventSpecToRaw(req.GetSpec())
	if err != nil {
		return nil, err
	}

	cr := &eventsv1alpha1.GitHubEvent{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-githubevent-",
			Namespace:    s.namespace,
		},
		Spec: eventsv1alpha1.GitHubEventSpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create github event: %v", err)
	}

	pb, err := githubEventToProto(cr)
	if err != nil {
		return nil, err
	}
	return &githubeventpb.CreateGitHubEventResponse{GithubEvent: pb}, nil
}

func (s *githubEventServer) GetGitHubEvent(
	ctx context.Context, req *githubeventpb.GetGitHubEventRequest,
) (*githubeventpb.GetGitHubEventResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := githubEventToProto(cr)
	if err != nil {
		return nil, err
	}
	return &githubeventpb.GetGitHubEventResponse{GithubEvent: pb}, nil
}

func (s *githubEventServer) ListGitHubEvents(
	ctx context.Context, req *githubeventpb.ListGitHubEventsRequest,
) (*githubeventpb.ListGitHubEventsResponse, error) {
	list := &eventsv1alpha1.GitHubEventList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list github events: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbEvents := make([]*githubeventpb.GitHubEvent, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := githubEventSpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, err
		}
		if req.Repository != nil && spec.GetRepository() != req.GetRepository() {
			continue
		}
		if req.EventType != nil && spec.GetEventType().GetType() != req.GetEventType().GetType() {
			continue
		}
		if req.Triggered != nil {
			st, err := githubEventStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, err
			}
			if st.GetTriggered() != req.GetTriggered() {
				continue
			}
		}

		pb, err := githubEventToProto(item)
		if err != nil {
			return nil, err
		}
		pbEvents = append(pbEvents, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, e := range pbEvents {
			if e.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbEvents) {
		start = len(pbEvents)
	}

	end := len(pbEvents)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &githubeventpb.ListGitHubEventsResponse{GithubEvents: pbEvents[start:end]}
	if end < len(pbEvents) {
		next := pbEvents[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *githubEventServer) DeleteGitHubEvent(
	ctx context.Context, req *githubeventpb.DeleteGitHubEventRequest,
) (*githubeventpb.DeleteGitHubEventResponse, error) {
	cr := &eventsv1alpha1.GitHubEvent{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete github event %q: %v", req.GetName(), err)
	}
	return &githubeventpb.DeleteGitHubEventResponse{Success: true}, nil
}

func (s *githubEventServer) WatchGitHubEvent(
	req *githubeventpb.WatchGitHubEventRequest, stream grpc.ServerStreamingServer[githubeventpb.WatchGitHubEventResponse],
) error {
	ctx := stream.Context()

	if req.GetName() != "" && req.GetLabelSelector() != "" {
		return status.Error(codes.InvalidArgument, "name and label_selector are mutually exclusive")
	}

	opts := []client.ListOption{client.InNamespace(s.namespace)}
	if sel := req.GetLabelSelector(); sel != "" {
		parsed, err := labels.Parse(sel)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "parse label_selector: %v", err)
		}
		// Real server-side filtering — the API server only streams
		// events for objects matching this selector, not the whole
		// namespace filtered client-side.
		opts = append(opts, client.MatchingLabelsSelector{Selector: parsed})
	}

	list := &eventsv1alpha1.GitHubEventList{}
	w, err := s.client.Watch(ctx, list, opts...)
	if err != nil {
		return status.Errorf(codes.Internal, "watch github events: %v", err)
	}
	defer w.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := event.Object.(*eventsv1alpha1.GitHubEvent)
			if !ok {
				continue
			}
			if name := req.GetName(); name != "" && obj.Name != name {
				continue
			}

			pb, err := githubEventToProto(obj)
			if err != nil {
				return err
			}

			evType := commonv1.EventType_EVENT_TYPE_MODIFIED
			switch event.Type {
			case watch.Added:
				evType = commonv1.EventType_EVENT_TYPE_ADDED
			case watch.Deleted:
				evType = commonv1.EventType_EVENT_TYPE_DELETED
			}

			err = stream.Send(&githubeventpb.WatchGitHubEventResponse{GithubEvent: pb, Type: evType})
			if err != nil {
				return err
			}
			if evType == commonv1.EventType_EVENT_TYPE_DELETED && req.GetName() != "" {
				return nil
			}
		}
	}
}
