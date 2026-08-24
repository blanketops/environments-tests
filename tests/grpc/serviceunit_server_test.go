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

// serviceUnitServer is a real ServiceUnitServiceServer implementation,
// following the same pattern as buildServer/deploymentServer/
// packageServer — see build_server_test.go's doc comment for the
// rationale. ServiceUnit is generated into the same Go package as Build
// (blanketops/environments/v1alpha1), so this reuses the buildpb alias
// rather than adding a colliding new one — see suite_test.go's import
// comment. No Trigger RPC, WatchServiceUnit streams indefinitely (no
// terminal phase).

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
	buildpb "github.com/blanketops/environments-contract/blanketops/environments/v1alpha1"
)

type serviceUnitServer struct {
	buildpb.UnimplementedServiceUnitServiceServer

	client    client.WithWatch
	namespace string
}

func serviceUnitSpecFromRaw(raw runtime.RawExtension) (*buildpb.ServiceUnitSpec, error) {
	spec := &buildpb.ServiceUnitSpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal service unit spec: %v", err)
	}
	return spec, nil
}

func serviceUnitSpecToRaw(spec *buildpb.ServiceUnitSpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal service unit spec: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func serviceUnitStatusFromRaw(raw runtime.RawExtension) (*buildpb.ServiceUnitStatus, error) {
	st := &buildpb.ServiceUnitStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal service unit status: %v", err)
	}
	return st, nil
}

func serviceUnitToProto(cr *environmentsv1alpha1.ServiceUnit) (*buildpb.ServiceUnit, error) {
	spec, err := serviceUnitSpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := serviceUnitStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &buildpb.ServiceUnit{
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

func (s *serviceUnitServer) get(ctx context.Context, name string) (*environmentsv1alpha1.ServiceUnit, error) {
	cr := &environmentsv1alpha1.ServiceUnit{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service unit %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get service unit %q: %v", name, err)
	}
	return cr, nil
}

func (s *serviceUnitServer) CreateServiceUnit(
	ctx context.Context, req *buildpb.CreateServiceUnitRequest,
) (*buildpb.CreateServiceUnitResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := serviceUnitSpecToRaw(req.GetSpec())
	if err != nil {
		return nil, err
	}

	cr := &environmentsv1alpha1.ServiceUnit{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-serviceunit-",
			Namespace:    s.namespace,
		},
		Spec: environmentsv1alpha1.ServiceUnitSpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create service unit: %v", err)
	}

	pb, err := serviceUnitToProto(cr)
	if err != nil {
		return nil, err
	}
	return &buildpb.CreateServiceUnitResponse{ServiceUnit: pb}, nil
}

func (s *serviceUnitServer) GetServiceUnit(
	ctx context.Context, req *buildpb.GetServiceUnitRequest,
) (*buildpb.GetServiceUnitResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := serviceUnitToProto(cr)
	if err != nil {
		return nil, err
	}
	return &buildpb.GetServiceUnitResponse{ServiceUnit: pb}, nil
}

func (s *serviceUnitServer) UpdateServiceUnit(
	ctx context.Context, req *buildpb.UpdateServiceUnitRequest,
) (*buildpb.UpdateServiceUnitResponse, error) {
	name := req.GetServiceUnit().GetMetadata().GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "service_unit.metadata.name is required")
	}
	cr, err := s.get(ctx, name)
	if err != nil {
		return nil, err
	}

	raw, err := serviceUnitSpecToRaw(req.GetServiceUnit().GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update service unit %q: %v", name, err)
	}

	pb, err := serviceUnitToProto(cr)
	if err != nil {
		return nil, err
	}
	return &buildpb.UpdateServiceUnitResponse{ServiceUnit: pb}, nil
}

func (s *serviceUnitServer) PatchServiceUnit(
	ctx context.Context, req *buildpb.PatchServiceUnitRequest,
) (*buildpb.PatchServiceUnitResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	current, err := serviceUnitToProto(cr)
	if err != nil {
		return nil, err
	}
	// UseProtoNames — see buildServer.PatchBuild for why.
	currentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal current service unit: %v", err)
	}

	merged, err := jsonpatch.MergePatch(currentJSON, []byte(req.GetPatch()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "apply merge patch: %v", err)
	}

	patched := &buildpb.ServiceUnit{}
	if err := protojson.Unmarshal(merged, patched); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshal patched service unit: %v", err)
	}

	raw, err := serviceUnitSpecToRaw(patched.GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update service unit %q: %v", req.GetName(), err)
	}

	pb, err := serviceUnitToProto(cr)
	if err != nil {
		return nil, err
	}
	return &buildpb.PatchServiceUnitResponse{ServiceUnit: pb}, nil
}

func (s *serviceUnitServer) ListServiceUnits(
	ctx context.Context, req *buildpb.ListServiceUnitsRequest,
) (*buildpb.ListServiceUnitsResponse, error) {
	list := &environmentsv1alpha1.ServiceUnitList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list service units: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbUnits := make([]*buildpb.ServiceUnit, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := serviceUnitSpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, err
		}
		if req.Type != nil && spec.GetType().GetType() != req.GetType().GetType() {
			continue
		}
		if req.AppType != nil && spec.GetAppType() != req.GetAppType() {
			continue
		}
		if req.Phase != nil {
			st, err := serviceUnitStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, err
			}
			if st.GetPhase().GetPhase() != req.GetPhase().GetPhase() {
				continue
			}
		}

		pb, err := serviceUnitToProto(item)
		if err != nil {
			return nil, err
		}
		pbUnits = append(pbUnits, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, u := range pbUnits {
			if u.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbUnits) {
		start = len(pbUnits)
	}

	end := len(pbUnits)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &buildpb.ListServiceUnitsResponse{ServiceUnits: pbUnits[start:end]}
	if end < len(pbUnits) {
		next := pbUnits[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *serviceUnitServer) DeleteServiceUnit(
	ctx context.Context, req *buildpb.DeleteServiceUnitRequest,
) (*buildpb.DeleteServiceUnitResponse, error) {
	cr := &environmentsv1alpha1.ServiceUnit{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete service unit %q: %v", req.GetName(), err)
	}
	return &buildpb.DeleteServiceUnitResponse{Success: true}, nil
}

func (s *serviceUnitServer) WatchServiceUnit(
	req *buildpb.WatchServiceUnitRequest, stream grpc.ServerStreamingServer[buildpb.WatchServiceUnitResponse],
) error {
	ctx := stream.Context()

	list := &environmentsv1alpha1.ServiceUnitList{}
	w, err := s.client.Watch(ctx, list, client.InNamespace(s.namespace))
	if err != nil {
		return status.Errorf(codes.Internal, "watch service units: %v", err)
	}
	defer w.Stop()

	// Service units have no terminal phase (see the proto's own comment
	// on WatchServiceUnit) — streams every event until the client
	// cancels, same as WatchDeployment/WatchPackage.
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := event.Object.(*environmentsv1alpha1.ServiceUnit)
			if !ok || obj.Name != req.GetName() {
				continue
			}

			pb, err := serviceUnitToProto(obj)
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

			if err := stream.Send(&buildpb.WatchServiceUnitResponse{ServiceUnit: pb, Type: evType}); err != nil {
				return err
			}
			if evType == commonv1.EventType_EVENT_TYPE_DELETED {
				return nil
			}
		}
	}
}
