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

// packageServer is a real PackageServiceServer implementation, following
// the same pattern as buildServer/deploymentServer — see
// build_server_test.go's doc comment for the rationale. PackageService
// has no Trigger RPC, and WatchPackage streams indefinitely like
// WatchDeployment (no terminal phase).

import (
	"context"
	"sort"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	deploymentpb "github.com/blanketops/environments-contract/blanketops/environments/v1"
)

type packageServer struct {
	deploymentpb.UnimplementedPackageServiceServer

	client    client.WithWatch
	namespace string
}

func packageSpecFromRaw(raw runtime.RawExtension) (*deploymentpb.PackageSpec, error) {
	spec := &deploymentpb.PackageSpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal package spec: %v", err)
	}
	return spec, nil
}

func packageSpecToRaw(spec *deploymentpb.PackageSpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal package spec: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func packageStatusFromRaw(raw runtime.RawExtension) (*deploymentpb.PackageStatus, error) {
	st := &deploymentpb.PackageStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal package status: %v", err)
	}
	return st, nil
}

func packageToProto(cr *environmentsv1alpha1.Package) (*deploymentpb.Package, error) {
	spec, err := packageSpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := packageStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.Package{
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

func (s *packageServer) get(ctx context.Context, name string) (*environmentsv1alpha1.Package, error) {
	cr := &environmentsv1alpha1.Package{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "package %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get package %q: %v", name, err)
	}
	return cr, nil
}

func (s *packageServer) CreatePackage(
	ctx context.Context, req *deploymentpb.CreatePackageRequest,
) (*deploymentpb.CreatePackageResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := packageSpecToRaw(req.GetSpec())
	if err != nil {
		return nil, err
	}

	cr := &environmentsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-package-",
			Namespace:    s.namespace,
		},
		Spec: environmentsv1alpha1.PackageSpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create package: %v", err)
	}

	pb, err := packageToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.CreatePackageResponse{Package: pb}, nil
}

func (s *packageServer) GetPackage(
	ctx context.Context, req *deploymentpb.GetPackageRequest,
) (*deploymentpb.GetPackageResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := packageToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.GetPackageResponse{Package: pb}, nil
}

func (s *packageServer) UpdatePackage(
	ctx context.Context, req *deploymentpb.UpdatePackageRequest,
) (*deploymentpb.UpdatePackageResponse, error) {
	name := req.GetPackage().GetMetadata().GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "package.metadata.name is required")
	}
	cr, err := s.get(ctx, name)
	if err != nil {
		return nil, err
	}

	raw, err := packageSpecToRaw(req.GetPackage().GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update package %q: %v", name, err)
	}

	pb, err := packageToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.UpdatePackageResponse{Package: pb}, nil
}

func (s *packageServer) PatchPackage(
	ctx context.Context, req *deploymentpb.PatchPackageRequest,
) (*deploymentpb.PatchPackageResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	current, err := packageToProto(cr)
	if err != nil {
		return nil, err
	}
	// UseProtoNames — see buildServer.PatchBuild for why.
	currentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal current package: %v", err)
	}

	merged, err := jsonpatch.MergePatch(currentJSON, []byte(req.GetPatch()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "apply merge patch: %v", err)
	}

	patched := &deploymentpb.Package{}
	if err := protojson.Unmarshal(merged, patched); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshal patched package: %v", err)
	}

	raw, err := packageSpecToRaw(patched.GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update package %q: %v", req.GetName(), err)
	}

	pb, err := packageToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.PatchPackageResponse{Package: pb}, nil
}

func (s *packageServer) ListPackages(
	ctx context.Context, req *deploymentpb.ListPackagesRequest,
) (*deploymentpb.ListPackagesResponse, error) {
	list := &environmentsv1alpha1.PackageList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list packages: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbPackages := make([]*deploymentpb.Package, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := packageSpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, err
		}
		if req.Enabled != nil && spec.GetEnabled() != req.GetEnabled() {
			continue
		}
		if req.Name != nil && spec.GetName() != req.GetName() {
			continue
		}
		if req.Phase != nil {
			st, err := packageStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, err
			}
			if st.GetPhase().GetPhase() != req.GetPhase().GetPhase() {
				continue
			}
		}

		pb, err := packageToProto(item)
		if err != nil {
			return nil, err
		}
		pbPackages = append(pbPackages, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, p := range pbPackages {
			if p.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbPackages) {
		start = len(pbPackages)
	}

	end := len(pbPackages)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &deploymentpb.ListPackagesResponse{Packages: pbPackages[start:end]}
	if end < len(pbPackages) {
		next := pbPackages[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *packageServer) DeletePackage(
	ctx context.Context, req *deploymentpb.DeletePackageRequest,
) (*deploymentpb.DeletePackageResponse, error) {
	cr := &environmentsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete package %q: %v", req.GetName(), err)
	}
	return &deploymentpb.DeletePackageResponse{Success: true}, nil
}

func (s *packageServer) WatchPackage(
	req *deploymentpb.WatchPackageRequest, stream grpc.ServerStreamingServer[deploymentpb.WatchPackageResponse],
) error {
	ctx := stream.Context()

	list := &environmentsv1alpha1.PackageList{}
	w, err := s.client.Watch(ctx, list, client.InNamespace(s.namespace))
	if err != nil {
		return status.Errorf(codes.Internal, "watch packages: %v", err)
	}
	defer w.Stop()

	// Packages have no terminal phase (see the proto's own comment on
	// WatchPackage) — this streams every event until the client cancels,
	// same as WatchDeployment.
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := event.Object.(*environmentsv1alpha1.Package)
			if !ok || obj.Name != req.GetName() {
				continue
			}

			pb, err := packageToProto(obj)
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

			if err := stream.Send(&deploymentpb.WatchPackageResponse{Package: pb, Type: evType}); err != nil {
				return err
			}
			if evType == commonv1.EventType_EVENT_TYPE_DELETED {
				return nil
			}
		}
	}
}
