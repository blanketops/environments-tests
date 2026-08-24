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

// buildServer is a real BuildServiceServer implementation backed by the
// real controller-runtime client — no production gRPC server exists
// anywhere in the blanketops family yet, so this is both the test seam
// and a reference for wiring the remaining services.
//
// The Build CR's spec.contract and status.contract fields are opaque
// runtime.RawExtension — Kubernetes never validates their contents, and
// no real domain implementation exists yet to give them fixed meaning.
// This server owns both sides of that opacity: it writes protojson-
// encoded BuildSpec/BuildStatus into Contract on the way in and decodes
// the same shape back out, so round-tripping is real and internally
// consistent even though nothing else in the cluster currently
// interprets that JSON.
//
// TriggerBuild cannot emit a real Shipwright BuildRun — Shipwright isn't
// installed against the clusters these tests run on (see
// dev_machine_k3s_setup / the kind cluster in run-tests.yml) — so it
// does the real part of its job (persist param overrides, advance
// status to Running with a generated execution ref) and stops there.

import (
	"context"
	"fmt"
	"sort"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	buildpb "github.com/blanketops/environments-contract/blanketops/environments/v1alpha1"
)

type buildServer struct {
	buildpb.UnimplementedBuildServiceServer

	client    client.WithWatch
	namespace string
}

func buildSpecFromRaw(raw runtime.RawExtension) (*buildpb.BuildSpec, error) {
	spec := &buildpb.BuildSpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, fmt.Errorf("unmarshal build spec: %w", err)
	}
	return spec, nil
}

func buildSpecToRaw(spec *buildpb.BuildSpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("marshal build spec: %w", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func buildStatusFromRaw(raw runtime.RawExtension) (*buildpb.BuildStatus, error) {
	st := &buildpb.BuildStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, fmt.Errorf("unmarshal build status: %w", err)
	}
	return st, nil
}

func buildStatusToRaw(st *buildpb.BuildStatus) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(st)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("marshal build status: %w", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func buildToProto(cr *environmentsv1alpha1.Build) (*buildpb.Build, error) {
	spec, err := buildSpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := buildStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &buildpb.Build{
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

func isTerminalBuildPhase(phase commonv1.BuildStatusPhase_BuildStatusPhase) bool {
	switch phase {
	case commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_SUCCEEDED,
		commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_FAILED,
		commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_CANCELLED:
		return true
	default:
		return false
	}
}

func mergeParams(base, overrides []*buildpb.Param) []*buildpb.Param {
	merged := append([]*buildpb.Param{}, base...)
	for _, o := range overrides {
		replaced := false
		for i, b := range merged {
			if b.GetName() == o.GetName() {
				merged[i] = o
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, o)
		}
	}
	return merged
}

func (s *buildServer) get(ctx context.Context, name string) (*environmentsv1alpha1.Build, error) {
	cr := &environmentsv1alpha1.Build{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "build %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get build %q: %v", name, err)
	}
	return cr, nil
}

func (s *buildServer) CreateBuild(
	ctx context.Context, req *buildpb.CreateBuildRequest,
) (*buildpb.CreateBuildResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := buildSpecToRaw(req.GetSpec())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	cr := &environmentsv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-build-",
			Namespace:    s.namespace,
		},
		Spec: environmentsv1alpha1.BuildSpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create build: %v", err)
	}

	pb, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &buildpb.CreateBuildResponse{Build: pb}, nil
}

func (s *buildServer) GetBuild(ctx context.Context, req *buildpb.GetBuildRequest) (*buildpb.GetBuildResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &buildpb.GetBuildResponse{Build: pb}, nil
}

func (s *buildServer) UpdateBuild(
	ctx context.Context, req *buildpb.UpdateBuildRequest,
) (*buildpb.UpdateBuildResponse, error) {
	name := req.GetBuild().GetMetadata().GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "build.metadata.name is required")
	}
	cr, err := s.get(ctx, name)
	if err != nil {
		return nil, err
	}

	raw, err := buildSpecToRaw(req.GetBuild().GetSpec())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update build %q: %v", name, err)
	}

	pb, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &buildpb.UpdateBuildResponse{Build: pb}, nil
}

func (s *buildServer) PatchBuild(
	ctx context.Context, req *buildpb.PatchBuildRequest,
) (*buildpb.PatchBuildResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	current, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// UseProtoNames so the base document's keys (max_attempts, not
	// maxAttempts) match the snake_case patch document's — the proto's
	// own documented patch example uses proto field names. Without this,
	// a merge patch adds a sibling key instead of overwriting, and
	// protojson.Unmarshal rejects the result as a duplicate field.
	currentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal current build: %v", err)
	}

	merged, err := jsonpatch.MergePatch(currentJSON, []byte(req.GetPatch()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "apply merge patch: %v", err)
	}

	patched := &buildpb.Build{}
	if err := protojson.Unmarshal(merged, patched); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshal patched build: %v", err)
	}

	raw, err := buildSpecToRaw(patched.GetSpec())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update build %q: %v", req.GetName(), err)
	}

	pb, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &buildpb.PatchBuildResponse{Build: pb}, nil
}

func (s *buildServer) ListBuilds(
	ctx context.Context, req *buildpb.ListBuildsRequest,
) (*buildpb.ListBuildsResponse, error) {
	list := &environmentsv1alpha1.BuildList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list builds: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbBuilds := make([]*buildpb.Build, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := buildSpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if req.Strategy != nil && spec.GetStrategy().GetName() != req.GetStrategy() {
			continue
		}
		if req.Phase != nil {
			st, err := buildStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if st.GetPhase().GetPhase() != req.GetPhase().GetPhase() {
				continue
			}
		}

		pb, err := buildToProto(item)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		pbBuilds = append(pbBuilds, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, b := range pbBuilds {
			if b.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbBuilds) {
		start = len(pbBuilds)
	}

	end := len(pbBuilds)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &buildpb.ListBuildsResponse{Builds: pbBuilds[start:end]}
	if end < len(pbBuilds) {
		next := pbBuilds[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *buildServer) DeleteBuild(
	ctx context.Context, req *buildpb.DeleteBuildRequest,
) (*buildpb.DeleteBuildResponse, error) {
	cr := &environmentsv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete build %q: %v", req.GetName(), err)
	}
	return &buildpb.DeleteBuildResponse{Success: true}, nil
}

func (s *buildServer) TriggerBuild(
	ctx context.Context, req *buildpb.TriggerBuildRequest,
) (*buildpb.TriggerBuildResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	if len(req.GetParams()) > 0 {
		spec, err := buildSpecFromRaw(cr.Spec.Contract)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		spec.Params = mergeParams(spec.GetParams(), req.GetParams())
		raw, err := buildSpecToRaw(spec)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		cr.Spec.Contract = raw
		if err := s.client.Update(ctx, cr); err != nil {
			return nil, status.Errorf(codes.Internal, "update build %q: %v", req.GetName(), err)
		}
	}

	execRef := fmt.Sprintf("%s-run-%d-%d", cr.Name, cr.Generation, time.Now().UnixNano())

	st, err := buildStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	st.ExecutionRef = execRef
	st.Phase = &commonv1.BuildStatusPhase{Phase: commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_RUNNING}
	st.StartTime = timestamppb.Now()

	rawStatus, err := buildStatusToRaw(st)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	cr.Status.Contract = rawStatus
	if err := s.client.Status().Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update build %q status: %v", req.GetName(), err)
	}

	pb, err := buildToProto(cr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &buildpb.TriggerBuildResponse{ExecutionRef: execRef, Build: pb}, nil
}

func (s *buildServer) WatchBuild(
	req *buildpb.WatchBuildRequest, stream grpc.ServerStreamingServer[buildpb.WatchBuildResponse],
) error {
	ctx := stream.Context()

	list := &environmentsv1alpha1.BuildList{}
	w, err := s.client.Watch(ctx, list, client.InNamespace(s.namespace))
	if err != nil {
		return status.Errorf(codes.Internal, "watch builds: %v", err)
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
			obj, ok := event.Object.(*environmentsv1alpha1.Build)
			if !ok || obj.Name != req.GetName() {
				continue
			}

			pb, err := buildToProto(obj)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}

			evType := commonv1.EventType_EVENT_TYPE_MODIFIED
			switch event.Type {
			case watch.Added:
				evType = commonv1.EventType_EVENT_TYPE_ADDED
			case watch.Deleted:
				evType = commonv1.EventType_EVENT_TYPE_DELETED
			}

			if err := stream.Send(&buildpb.WatchBuildResponse{Build: pb, Type: evType}); err != nil {
				return err
			}

			if evType == commonv1.EventType_EVENT_TYPE_DELETED {
				return nil
			}
			if isTerminalBuildPhase(pb.GetStatus().GetPhase().GetPhase()) {
				return nil
			}
		}
	}
}
