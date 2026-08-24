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

// gitRepositoryServer is a real GitRepositoryServiceServer
// implementation, following the same pattern as buildServer and
// siblings — see build_server_test.go's doc comment for the rationale.
//
// SyncGitRepository ("force webhook re-registration against the
// provider") can't do the real thing — no GitHub/Crossplane webhook
// integration exists on the clusters these tests run against, same
// limitation as TriggerBuild not emitting a real Shipwright BuildRun.
// It does the honest part of its job (touch last_updated_at, clear any
// prior failure reason) and stops there.
//
// The proto's WatchGitRepository comment says it "closes when the git
// repository reaches a terminal phase (SUCCEEDED, FAILED, CANCELLED)"
// — that's Build's phase vocabulary copy-pasted in; GitRepositoryStatus
// actually only has a bool Ready + Reason, no such phases. Treating
// ready=true as the terminal signal is the closest honest reading of
// that comment against the real message shape.
//
// The stub this replaces only named 7 of this service's 8 RPCs as
// tests (Update/Patch/Sync were never stubbed even though the proto has
// them) — implementing the full real surface here, and
// gitrepository_grpc_test.go adds tests for the three that were missing
// rather than leaving them silently uncovered.

import (
	"context"
	"sort"

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

	sourcesv1alpha1 "github.com/blanketops/environments-api/api/sources/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	gitrepositorypb "github.com/blanketops/environments-contract/blanketops/sources/v1alpha1"
)

type gitRepositoryServer struct {
	gitrepositorypb.UnimplementedGitRepositoryServiceServer

	client    client.WithWatch
	namespace string
}

func gitRepositorySpecFromRaw(raw runtime.RawExtension) (*gitrepositorypb.GitRepositorySpec, error) {
	spec := &gitrepositorypb.GitRepositorySpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal git repository spec: %v", err)
	}
	return spec, nil
}

func gitRepositorySpecToRaw(spec *gitrepositorypb.GitRepositorySpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal git repository spec: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func gitRepositoryStatusFromRaw(raw runtime.RawExtension) (*gitrepositorypb.GitRepositoryStatus, error) {
	st := &gitrepositorypb.GitRepositoryStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal git repository status: %v", err)
	}
	return st, nil
}

func gitRepositoryStatusToRaw(st *gitrepositorypb.GitRepositoryStatus) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(st)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal git repository status: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func gitRepositoryToProto(cr *sourcesv1alpha1.GitRepository) (*gitrepositorypb.GitRepository, error) {
	spec, err := gitRepositorySpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := gitRepositoryStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.GitRepository{
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

func (s *gitRepositoryServer) get(ctx context.Context, name string) (*sourcesv1alpha1.GitRepository, error) {
	cr := &sourcesv1alpha1.GitRepository{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "git repository %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get git repository %q: %v", name, err)
	}
	return cr, nil
}

func (s *gitRepositoryServer) CreateGitRepository(
	ctx context.Context, req *gitrepositorypb.CreateGitRepositoryRequest,
) (*gitrepositorypb.CreateGitRepositoryResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := gitRepositorySpecToRaw(req.GetSpec())
	if err != nil {
		return nil, err
	}

	cr := &sourcesv1alpha1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-gitrepository-",
			Namespace:    s.namespace,
		},
		Spec: sourcesv1alpha1.GitRepositorySpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create git repository: %v", err)
	}

	pb, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.CreateGitRepositoryResponse{GitRepository: pb}, nil
}

func (s *gitRepositoryServer) GetGitRepository(
	ctx context.Context, req *gitrepositorypb.GetGitRepositoryRequest,
) (*gitrepositorypb.GetGitRepositoryResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.GetGitRepositoryResponse{GitRepository: pb}, nil
}

func (s *gitRepositoryServer) UpdateGitRepository(
	ctx context.Context, req *gitrepositorypb.UpdateGitRepositoryRequest,
) (*gitrepositorypb.UpdateGitRepositoryResponse, error) {
	name := req.GetGitRepository().GetMetadata().GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "git_repository.metadata.name is required")
	}
	cr, err := s.get(ctx, name)
	if err != nil {
		return nil, err
	}

	raw, err := gitRepositorySpecToRaw(req.GetGitRepository().GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update git repository %q: %v", name, err)
	}

	pb, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.UpdateGitRepositoryResponse{GitRepository: pb}, nil
}

func (s *gitRepositoryServer) PatchGitRepository(
	ctx context.Context, req *gitrepositorypb.PatchGitRepositoryRequest,
) (*gitrepositorypb.PatchGitRepositoryResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	current, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	// UseProtoNames — see buildServer.PatchBuild for why.
	currentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal current git repository: %v", err)
	}

	merged, err := jsonpatch.MergePatch(currentJSON, []byte(req.GetPatch()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "apply merge patch: %v", err)
	}

	patched := &gitrepositorypb.GitRepository{}
	if err := protojson.Unmarshal(merged, patched); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshal patched git repository: %v", err)
	}

	raw, err := gitRepositorySpecToRaw(patched.GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update git repository %q: %v", req.GetName(), err)
	}

	pb, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.PatchGitRepositoryResponse{GitRepository: pb}, nil
}

func (s *gitRepositoryServer) ListGitRepositories(
	ctx context.Context, req *gitrepositorypb.ListGitRepositoriesRequest,
) (*gitrepositorypb.ListGitRepositoriesResponse, error) {
	list := &sourcesv1alpha1.GitRepositoryList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list git repositories: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbRepos := make([]*gitrepositorypb.GitRepository, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := gitRepositorySpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, err
		}
		if req.Provider != nil && spec.GetProvider().GetProvider() != req.GetProvider().GetProvider() {
			continue
		}
		if req.Owner != nil && spec.GetRepository().GetOwner() != req.GetOwner() {
			continue
		}
		if req.Ready != nil {
			st, err := gitRepositoryStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, err
			}
			if st.GetReady() != req.GetReady() {
				continue
			}
		}

		pb, err := gitRepositoryToProto(item)
		if err != nil {
			return nil, err
		}
		pbRepos = append(pbRepos, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, r := range pbRepos {
			if r.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbRepos) {
		start = len(pbRepos)
	}

	end := len(pbRepos)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &gitrepositorypb.ListGitRepositoriesResponse{GitRepositories: pbRepos[start:end]}
	if end < len(pbRepos) {
		next := pbRepos[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *gitRepositoryServer) DeleteGitRepository(
	ctx context.Context, req *gitrepositorypb.DeleteGitRepositoryRequest,
) (*gitrepositorypb.DeleteGitRepositoryResponse, error) {
	cr := &sourcesv1alpha1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete git repository %q: %v", req.GetName(), err)
	}
	return &gitrepositorypb.DeleteGitRepositoryResponse{Success: true}, nil
}

func (s *gitRepositoryServer) SyncGitRepository(
	ctx context.Context, req *gitrepositorypb.SyncGitRepositoryRequest,
) (*gitrepositorypb.SyncGitRepositoryResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	st, err := gitRepositoryStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	st.Reason = ""
	st.LastUpdatedAt = timestamppb.Now()

	raw, err := gitRepositoryStatusToRaw(st)
	if err != nil {
		return nil, err
	}
	cr.Status.Contract = raw
	if err := s.client.Status().Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update git repository %q status: %v", req.GetName(), err)
	}

	pb, err := gitRepositoryToProto(cr)
	if err != nil {
		return nil, err
	}
	return &gitrepositorypb.SyncGitRepositoryResponse{GitRepository: pb}, nil
}

func (s *gitRepositoryServer) WatchGitRepository(
	req *gitrepositorypb.WatchGitRepositoryRequest,
	stream grpc.ServerStreamingServer[gitrepositorypb.WatchGitRepositoryResponse],
) error {
	ctx := stream.Context()

	list := &sourcesv1alpha1.GitRepositoryList{}
	w, err := s.client.Watch(ctx, list, client.InNamespace(s.namespace))
	if err != nil {
		return status.Errorf(codes.Internal, "watch git repositories: %v", err)
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
			obj, ok := event.Object.(*sourcesv1alpha1.GitRepository)
			if !ok || obj.Name != req.GetName() {
				continue
			}

			pb, err := gitRepositoryToProto(obj)
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

			resp := &gitrepositorypb.WatchGitRepositoryResponse{GitRepository: pb, Type: evType}
			if err := stream.Send(resp); err != nil {
				return err
			}

			if evType == commonv1.EventType_EVENT_TYPE_DELETED {
				return nil
			}
			// See this file's doc comment: ready=true is treated as the
			// terminal signal the proto's Watch comment gestures at.
			if pb.GetStatus().GetReady() {
				return nil
			}
		}
	}
}
