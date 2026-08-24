/*
Copyright 2025 The BlanketOps Authors.
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

// DeploymentService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1/deployment.proto
// Service:    DeploymentService
//
// Each test calls the real DeploymentService over a real gRPC connection
// (in-process, see TestMain in suite_test.go) — no fakes, no mocks.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	deploymentpb "github.com/blanketops/environments-contract/blanketops/environments/v1"
)

func newDeploymentSpec() *deploymentpb.DeploymentSpec {
	return &deploymentpb.DeploymentSpec{
		ServiceUnits: []string{"for-kaniko-app-api"},
		Runtime: &commonv1.DeploymentRuntime{
			Runtime: commonv1.DeploymentRuntime_DEPLOYMENT_RUNTIME_KUBERNETES_CONTAINER,
		},
		ImageAutomation: false,
		ManifestsRepo: &deploymentpb.ManifestsRepo{
			Url:         "git@github.com:ntlaletsi70/for-kaniko-app-manifests.git",
			Ref:         "main",
			CloneSecret: "git-ssh-credentials",
			Strategy:    "kustomize",
			Path:        "./overlays/dev",
		},
		ReconciliationStrategy: &commonv1.ReconciliationStrategy{
			Strategy: commonv1.ReconciliationStrategy_RECONCILIATION_STRATEGY_KUSTOMIZE,
		},
	}
}

// createTestDeployment creates a Deployment via the real gRPC service and
// registers its cleanup, returning the created proto Deployment.
func createTestDeployment(t *testing.T, c deploymentpb.DeploymentServiceClient) *deploymentpb.Deployment {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreateDeployment(ctx, &deploymentpb.CreateDeploymentRequest{Spec: newDeploymentSpec()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetDeployment())

	name := resp.GetDeployment().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteDeployment(context.Background(), &deploymentpb.DeleteDeploymentRequest{Name: name})
	})

	return resp.GetDeployment()
}

func TestDeploymentService_CreateDeployment(t *testing.T) {
	c := deploymentClient(t)

	deployment := createTestDeployment(t, c)

	assert.NotEmpty(t, deployment.GetMetadata().GetName())
	assert.NotEmpty(t, deployment.GetMetadata().GetId())
	assert.Equal(t, []string{"for-kaniko-app-api"}, deployment.GetSpec().GetServiceUnits())
	assert.Equal(t,
		commonv1.DeploymentRuntime_DEPLOYMENT_RUNTIME_KUBERNETES_CONTAINER,
		deployment.GetSpec().GetRuntime().GetRuntime(),
	)
}

func TestDeploymentService_GetDeployment(t *testing.T) {
	c := deploymentClient(t)
	created := createTestDeployment(t, c)

	resp, err := c.GetDeployment(context.Background(), &deploymentpb.GetDeploymentRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetDeployment())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetDeployment().GetMetadata().GetName())
	assert.Equal(t, created.GetSpec().GetServiceUnits(), resp.GetDeployment().GetSpec().GetServiceUnits())
}

func TestDeploymentService_UpdateDeployment(t *testing.T) {
	c := deploymentClient(t)
	ctx := context.Background()
	created := createTestDeployment(t, c)

	newSpec := newDeploymentSpec()
	newSpec.ServiceUnits = []string{"for-kaniko-app-api", "for-kaniko-app-worker"}
	newSpec.ImageAutomation = true

	resp, err := c.UpdateDeployment(ctx, &deploymentpb.UpdateDeploymentRequest{
		Deployment: &deploymentpb.Deployment{
			Metadata: &commonv1.Metadata{Name: created.GetMetadata().GetName()},
			Spec:     newSpec,
		},
	})
	require.NoError(t, err)
	wantUnits := []string{"for-kaniko-app-api", "for-kaniko-app-worker"}
	assert.Equal(t, wantUnits, resp.GetDeployment().GetSpec().GetServiceUnits())
	assert.True(t, resp.GetDeployment().GetSpec().GetImageAutomation())

	getResp, err := c.GetDeployment(ctx, &deploymentpb.GetDeploymentRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	assert.True(t, getResp.GetDeployment().GetSpec().GetImageAutomation())
}

func TestDeploymentService_PatchDeployment(t *testing.T) {
	c := deploymentClient(t)
	ctx := context.Background()
	created := createTestDeployment(t, c)

	patch := `{"spec":{"image_automation":true}}`
	resp, err := c.PatchDeployment(ctx, &deploymentpb.PatchDeploymentRequest{
		Name:  created.GetMetadata().GetName(),
		Patch: patch,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetDeployment())

	assert.True(t, resp.GetDeployment().GetSpec().GetImageAutomation())
	// Fields outside the patch survive untouched.
	assert.Equal(t, []string{"for-kaniko-app-api"}, resp.GetDeployment().GetSpec().GetServiceUnits())
	assert.Equal(t, "kustomize", resp.GetDeployment().GetSpec().GetManifestsRepo().GetStrategy())
}

func TestDeploymentService_ListDeployments(t *testing.T) {
	c := deploymentClient(t)
	ctx := context.Background()
	created := createTestDeployment(t, c)

	resp, err := c.ListDeployments(ctx, &deploymentpb.ListDeploymentsRequest{})
	require.NoError(t, err)

	var found bool
	for _, d := range resp.GetDeployments() {
		if d.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListDeployments to include the created deployment")

	wantRuntime := &commonv1.DeploymentRuntime{Runtime: commonv1.DeploymentRuntime_DEPLOYMENT_RUNTIME_KUBERNETES_CONTAINER}
	filtered, err := c.ListDeployments(ctx, &deploymentpb.ListDeploymentsRequest{Runtime: wantRuntime})
	require.NoError(t, err)
	for _, d := range filtered.GetDeployments() {
		gotRuntime := d.GetSpec().GetRuntime().GetRuntime()
		assert.Equal(t, commonv1.DeploymentRuntime_DEPLOYMENT_RUNTIME_KUBERNETES_CONTAINER, gotRuntime)
	}
}

func TestDeploymentService_ListDeployments_Pagination(t *testing.T) {
	c := deploymentClient(t)

	for range 3 {
		createTestDeployment(t, c)
	}

	pageSize := int32(1)
	page1, err := c.ListDeployments(context.Background(), &deploymentpb.ListDeploymentsRequest{PageSize: &pageSize})
	require.NoError(t, err)
	assert.Len(t, page1.GetDeployments(), 1)
	require.NotNil(t, page1.NextPageToken)
	assert.NotEmpty(t, page1.GetNextPageToken())

	page2, err := c.ListDeployments(context.Background(), &deploymentpb.ListDeploymentsRequest{
		PageSize:  &pageSize,
		PageToken: page1.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, page2.GetDeployments(), 1)
	assert.NotEqual(t,
		page1.GetDeployments()[0].GetMetadata().GetName(),
		page2.GetDeployments()[0].GetMetadata().GetName(),
	)
}

func TestDeploymentService_DeleteDeployment(t *testing.T) {
	c := deploymentClient(t)
	ctx := context.Background()

	resp, err := c.CreateDeployment(ctx, &deploymentpb.CreateDeploymentRequest{Spec: newDeploymentSpec()})
	require.NoError(t, err)
	name := resp.GetDeployment().GetMetadata().GetName()

	delResp, err := c.DeleteDeployment(ctx, &deploymentpb.DeleteDeploymentRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetDeployment(ctx, &deploymentpb.GetDeploymentRequest{Name: name})
	assert.Error(t, err, "expected the deleted deployment to be gone")
}

func TestDeploymentService_WatchDeployment(t *testing.T) {
	c := deploymentClient(t)
	created := createTestDeployment(t, c)
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchDeployment(ctx, &deploymentpb.WatchDeploymentRequest{Name: name})
	require.NoError(t, err)

	// Deployments stream indefinitely (no terminal phase) — drive a
	// PENDING -> READY transition directly against the cluster, the same
	// way tests/fake manually calls Reconcile() in the absence of a real
	// controller, and cancel once we've observed it. PatchDeployment only
	// ever writes .spec (see deploymentServer.PatchDeployment), so status
	// has to be driven through k8sClient directly, same as WatchBuild's
	// test does.
	go func() {
		time.Sleep(200 * time.Millisecond)

		cr := &environmentsv1alpha1.Deployment{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := deploymentStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Phase = &commonv1.DeploymentPhase{Phase: commonv1.DeploymentPhase_DEPLOYMENT_PHASE_READY}
		raw, err := protojson.Marshal(st)
		if err != nil {
			return
		}
		cr.Status.Contract = runtime.RawExtension{Raw: raw}
		_ = k8sClient.Status().Update(ctx, cr)
	}()

	var sawReady bool
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		if event.GetDeployment().GetStatus().GetPhase().GetPhase() == commonv1.DeploymentPhase_DEPLOYMENT_PHASE_READY {
			sawReady = true
			cancel()
			break
		}
	}
	assert.True(t, sawReady, "expected WatchDeployment to deliver the Ready transition")
}

func TestDeploymentService_MultiUnitDeployment(t *testing.T) {
	c := deploymentClient(t)

	spec := newDeploymentSpec()
	spec.ServiceUnits = []string{"for-kaniko-app-api", "for-kaniko-app-worker"}
	spec.ImageAutomation = true

	resp, err := c.CreateDeployment(context.Background(), &deploymentpb.CreateDeploymentRequest{Spec: spec})
	require.NoError(t, err)
	name := resp.GetDeployment().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteDeployment(context.Background(), &deploymentpb.DeleteDeploymentRequest{Name: name})
	})

	wantUnits := []string{"for-kaniko-app-api", "for-kaniko-app-worker"}
	assert.Equal(t, wantUnits, resp.GetDeployment().GetSpec().GetServiceUnits())
	assert.True(t, resp.GetDeployment().GetSpec().GetImageAutomation())
}
