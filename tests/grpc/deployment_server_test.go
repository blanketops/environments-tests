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

// deploymentServer is a real DeploymentServiceServer implementation,
// following the same pattern as buildServer in build_server_test.go —
// see that file's doc comment for the rationale (opaque Contract, no
// production gRPC server to point at, real K8s CRUD + real merge patch
// + real k8s watch). DeploymentService has no Trigger RPC, and
// WatchDeployment has no terminal phase — deployments stream until the
// client cancels, not until a phase like Succeeded is reached.

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

type deploymentServer struct {
	deploymentpb.UnimplementedDeploymentServiceServer

	client    client.WithWatch
	namespace string
}

func deploymentSpecFromRaw(raw runtime.RawExtension) (*deploymentpb.DeploymentSpec, error) {
	spec := &deploymentpb.DeploymentSpec{}
	if len(raw.Raw) == 0 {
		return spec, nil
	}
	if err := protojson.Unmarshal(raw.Raw, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal deployment spec: %v", err)
	}
	return spec, nil
}

func deploymentSpecToRaw(spec *deploymentpb.DeploymentSpec) (runtime.RawExtension, error) {
	data, err := protojson.Marshal(spec)
	if err != nil {
		return runtime.RawExtension{}, status.Errorf(codes.Internal, "marshal deployment spec: %v", err)
	}
	return runtime.RawExtension{Raw: data}, nil
}

func deploymentStatusFromRaw(raw runtime.RawExtension) (*deploymentpb.DeploymentStatus, error) {
	st := &deploymentpb.DeploymentStatus{}
	if len(raw.Raw) == 0 {
		return st, nil
	}
	if err := protojson.Unmarshal(raw.Raw, st); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal deployment status: %v", err)
	}
	return st, nil
}

func deploymentToProto(cr *environmentsv1alpha1.Deployment) (*deploymentpb.Deployment, error) {
	spec, err := deploymentSpecFromRaw(cr.Spec.Contract)
	if err != nil {
		return nil, err
	}
	st, err := deploymentStatusFromRaw(cr.Status.Contract)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.Deployment{
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

func (s *deploymentServer) get(ctx context.Context, name string) (*environmentsv1alpha1.Deployment, error) {
	cr := &environmentsv1alpha1.Deployment{}
	key := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, key, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "deployment %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "get deployment %q: %v", name, err)
	}
	return cr, nil
}

func (s *deploymentServer) CreateDeployment(
	ctx context.Context, req *deploymentpb.CreateDeploymentRequest,
) (*deploymentpb.CreateDeploymentResponse, error) {
	if req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	raw, err := deploymentSpecToRaw(req.GetSpec())
	if err != nil {
		return nil, err
	}

	cr := &environmentsv1alpha1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grpc-deployment-",
			Namespace:    s.namespace,
		},
		Spec: environmentsv1alpha1.DeploymentSpec{Contract: raw},
	}
	if err := s.client.Create(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "create deployment: %v", err)
	}

	pb, err := deploymentToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.CreateDeploymentResponse{Deployment: pb}, nil
}

func (s *deploymentServer) GetDeployment(
	ctx context.Context, req *deploymentpb.GetDeploymentRequest,
) (*deploymentpb.GetDeploymentResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	pb, err := deploymentToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.GetDeploymentResponse{Deployment: pb}, nil
}

func (s *deploymentServer) UpdateDeployment(
	ctx context.Context, req *deploymentpb.UpdateDeploymentRequest,
) (*deploymentpb.UpdateDeploymentResponse, error) {
	name := req.GetDeployment().GetMetadata().GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "deployment.metadata.name is required")
	}
	cr, err := s.get(ctx, name)
	if err != nil {
		return nil, err
	}

	raw, err := deploymentSpecToRaw(req.GetDeployment().GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update deployment %q: %v", name, err)
	}

	pb, err := deploymentToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.UpdateDeploymentResponse{Deployment: pb}, nil
}

func (s *deploymentServer) PatchDeployment(
	ctx context.Context, req *deploymentpb.PatchDeploymentRequest,
) (*deploymentpb.PatchDeploymentResponse, error) {
	cr, err := s.get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}

	current, err := deploymentToProto(cr)
	if err != nil {
		return nil, err
	}
	// UseProtoNames — see buildServer.PatchBuild for why: the patch
	// documents in this suite use proto field names (snake_case), and
	// the base document must match or a merge patch adds a sibling key
	// instead of overwriting it.
	currentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal current deployment: %v", err)
	}

	merged, err := jsonpatch.MergePatch(currentJSON, []byte(req.GetPatch()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "apply merge patch: %v", err)
	}

	patched := &deploymentpb.Deployment{}
	if err := protojson.Unmarshal(merged, patched); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshal patched deployment: %v", err)
	}

	raw, err := deploymentSpecToRaw(patched.GetSpec())
	if err != nil {
		return nil, err
	}
	cr.Spec.Contract = raw
	if err := s.client.Update(ctx, cr); err != nil {
		return nil, status.Errorf(codes.Internal, "update deployment %q: %v", req.GetName(), err)
	}

	pb, err := deploymentToProto(cr)
	if err != nil {
		return nil, err
	}
	return &deploymentpb.PatchDeploymentResponse{Deployment: pb}, nil
}

func (s *deploymentServer) ListDeployments(
	ctx context.Context, req *deploymentpb.ListDeploymentsRequest,
) (*deploymentpb.ListDeploymentsResponse, error) {
	list := &environmentsv1alpha1.DeploymentList{}
	if err := s.client.List(ctx, list, client.InNamespace(s.namespace)); err != nil {
		return nil, status.Errorf(codes.Internal, "list deployments: %v", err)
	}

	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	pbDeployments := make([]*deploymentpb.Deployment, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]

		spec, err := deploymentSpecFromRaw(item.Spec.Contract)
		if err != nil {
			return nil, err
		}
		if req.Runtime != nil && spec.GetRuntime().GetRuntime() != req.GetRuntime().GetRuntime() {
			continue
		}
		if req.Phase != nil {
			st, err := deploymentStatusFromRaw(item.Status.Contract)
			if err != nil {
				return nil, err
			}
			if st.GetPhase().GetPhase() != req.GetPhase().GetPhase() {
				continue
			}
		}

		pb, err := deploymentToProto(item)
		if err != nil {
			return nil, err
		}
		pbDeployments = append(pbDeployments, pb)
	}

	start := 0
	if token := req.GetPageToken(); token != "" {
		for i, d := range pbDeployments {
			if d.GetMetadata().GetName() > token {
				start = i
				break
			}
			start = i + 1
		}
	}
	if start > len(pbDeployments) {
		start = len(pbDeployments)
	}

	end := len(pbDeployments)
	if size := req.GetPageSize(); size > 0 && start+int(size) < end {
		end = start + int(size)
	}

	resp := &deploymentpb.ListDeploymentsResponse{Deployments: pbDeployments[start:end]}
	if end < len(pbDeployments) {
		next := pbDeployments[end-1].GetMetadata().GetName()
		resp.NextPageToken = &next
	}
	return resp, nil
}

func (s *deploymentServer) DeleteDeployment(
	ctx context.Context, req *deploymentpb.DeleteDeploymentRequest,
) (*deploymentpb.DeleteDeploymentResponse, error) {
	cr := &environmentsv1alpha1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: req.GetName(), Namespace: s.namespace},
	}
	if err := s.client.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "delete deployment %q: %v", req.GetName(), err)
	}
	return &deploymentpb.DeleteDeploymentResponse{Success: true}, nil
}

func (s *deploymentServer) WatchDeployment(
	req *deploymentpb.WatchDeploymentRequest, stream grpc.ServerStreamingServer[deploymentpb.WatchDeploymentResponse],
) error {
	ctx := stream.Context()

	list := &environmentsv1alpha1.DeploymentList{}
	w, err := s.client.Watch(ctx, list, client.InNamespace(s.namespace))
	if err != nil {
		return status.Errorf(codes.Internal, "watch deployments: %v", err)
	}
	defer w.Stop()

	// Deployments have no terminal phase (see the proto's own comment on
	// WatchDeployment) — this streams every event for the named object
	// until the client cancels, unlike WatchBuild which stops itself.
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := event.Object.(*environmentsv1alpha1.Deployment)
			if !ok || obj.Name != req.GetName() {
				continue
			}

			pb, err := deploymentToProto(obj)
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

			if err := stream.Send(&deploymentpb.WatchDeploymentResponse{Deployment: pb, Type: evType}); err != nil {
				return err
			}
			if evType == commonv1.EventType_EVENT_TYPE_DELETED {
				return nil
			}
		}
	}
}
