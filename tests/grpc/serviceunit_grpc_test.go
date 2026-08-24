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

// ServiceUnitService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1alpha1/serviceunit.proto
// Service:    ServiceUnitService
//
// Each test calls the real ServiceUnitService over a real gRPC
// connection (in-process, see TestMain in suite_test.go) — no fakes,
// no mocks.

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
	buildpb "github.com/blanketops/environments-contract/blanketops/environments/v1alpha1"
)

func newStaticServiceUnitSpec() *buildpb.ServiceUnitSpec {
	return &buildpb.ServiceUnitSpec{
		Type:          &commonv1.ServiceUnitType{Type: commonv1.ServiceUnitType_SERVICE_UNIT_TYPE_STATIC},
		Image:         "docker.io/nkanyezisolutions/for-kaniko-app:master",
		ContainerPort: 8080,
		Size:          2,
		AppType:       "web",
		StackType:     "nodejs",
	}
}

func newBuildServiceUnitSpec() *buildpb.ServiceUnitSpec {
	return &buildpb.ServiceUnitSpec{
		Type:          &commonv1.ServiceUnitType{Type: commonv1.ServiceUnitType_SERVICE_UNIT_TYPE_BUILD},
		BuildRef:      &buildpb.BuildReference{Name: "for-kaniko-app-build"},
		ContainerPort: 9000,
		Size:          1,
		AppType:       "worker",
		StackType:     "python",
	}
}

// createTestServiceUnit creates a ServiceUnit via the real gRPC service
// and registers its cleanup, returning the created proto ServiceUnit.
func createTestServiceUnit(
	t *testing.T, c buildpb.ServiceUnitServiceClient, spec *buildpb.ServiceUnitSpec,
) *buildpb.ServiceUnit {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreateServiceUnit(ctx, &buildpb.CreateServiceUnitRequest{Spec: spec})
	require.NoError(t, err)
	require.NotNil(t, resp.GetServiceUnit())

	name := resp.GetServiceUnit().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteServiceUnit(context.Background(), &buildpb.DeleteServiceUnitRequest{Name: name})
	})

	return resp.GetServiceUnit()
}

func TestServiceUnitService_CreateServiceUnit_Static(t *testing.T) {
	c := serviceUnitClient(t)

	su := createTestServiceUnit(t, c, newStaticServiceUnitSpec())

	assert.Equal(t, commonv1.ServiceUnitType_SERVICE_UNIT_TYPE_STATIC, su.GetSpec().GetType().GetType())
	assert.NotEmpty(t, su.GetSpec().GetImage())
	assert.Nil(t, su.GetSpec().GetBuildRef())
}

func TestServiceUnitService_CreateServiceUnit_Build(t *testing.T) {
	c := serviceUnitClient(t)

	su := createTestServiceUnit(t, c, newBuildServiceUnitSpec())

	assert.Equal(t, commonv1.ServiceUnitType_SERVICE_UNIT_TYPE_BUILD, su.GetSpec().GetType().GetType())
	assert.NotNil(t, su.GetSpec().GetBuildRef())
	assert.Empty(t, su.GetSpec().GetImage())
}

func TestServiceUnitService_GetServiceUnit(t *testing.T) {
	c := serviceUnitClient(t)
	created := createTestServiceUnit(t, c, newStaticServiceUnitSpec())

	resp, err := c.GetServiceUnit(context.Background(), &buildpb.GetServiceUnitRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetServiceUnit())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetServiceUnit().GetMetadata().GetName())
	assert.Equal(t, created.GetSpec().GetImage(), resp.GetServiceUnit().GetSpec().GetImage())
}

func TestServiceUnitService_UpdateServiceUnit(t *testing.T) {
	c := serviceUnitClient(t)
	ctx := context.Background()
	created := createTestServiceUnit(t, c, newStaticServiceUnitSpec())

	newSpec := newStaticServiceUnitSpec()
	newSpec.Size = 4

	resp, err := c.UpdateServiceUnit(ctx, &buildpb.UpdateServiceUnitRequest{
		ServiceUnit: &buildpb.ServiceUnit{
			Metadata: &commonv1.Metadata{Name: created.GetMetadata().GetName()},
			Spec:     newSpec,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(4), resp.GetServiceUnit().GetSpec().GetSize())

	getResp, err := c.GetServiceUnit(ctx, &buildpb.GetServiceUnitRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	assert.Equal(t, int32(4), getResp.GetServiceUnit().GetSpec().GetSize())
}

func TestServiceUnitService_PatchServiceUnit(t *testing.T) {
	c := serviceUnitClient(t)
	ctx := context.Background()
	created := createTestServiceUnit(t, c, newStaticServiceUnitSpec())

	patch := `{"spec":{"size":3}}`
	resp, err := c.PatchServiceUnit(ctx, &buildpb.PatchServiceUnitRequest{
		Name:  created.GetMetadata().GetName(),
		Patch: patch,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetServiceUnit())

	assert.Equal(t, int32(3), resp.GetServiceUnit().GetSpec().GetSize())
	// Fields outside the patch survive untouched.
	assert.Equal(t, int32(8080), resp.GetServiceUnit().GetSpec().GetContainerPort())
	assert.Equal(t, "web", resp.GetServiceUnit().GetSpec().GetAppType())
}

func TestServiceUnitService_ListServiceUnits(t *testing.T) {
	c := serviceUnitClient(t)
	ctx := context.Background()
	created := createTestServiceUnit(t, c, newStaticServiceUnitSpec())

	resp, err := c.ListServiceUnits(ctx, &buildpb.ListServiceUnitsRequest{})
	require.NoError(t, err)

	var found bool
	for _, u := range resp.GetServiceUnits() {
		if u.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListServiceUnits to include the created service unit")

	appType := "web"
	filtered, err := c.ListServiceUnits(ctx, &buildpb.ListServiceUnitsRequest{AppType: &appType})
	require.NoError(t, err)
	for _, u := range filtered.GetServiceUnits() {
		assert.Equal(t, "web", u.GetSpec().GetAppType())
	}
}

func TestServiceUnitService_ListServiceUnits_Pagination(t *testing.T) {
	c := serviceUnitClient(t)

	for range 3 {
		createTestServiceUnit(t, c, newStaticServiceUnitSpec())
	}

	pageSize := int32(1)
	page1, err := c.ListServiceUnits(context.Background(), &buildpb.ListServiceUnitsRequest{PageSize: &pageSize})
	require.NoError(t, err)
	assert.Len(t, page1.GetServiceUnits(), 1)
	require.NotNil(t, page1.NextPageToken)
	assert.NotEmpty(t, page1.GetNextPageToken())

	page2, err := c.ListServiceUnits(context.Background(), &buildpb.ListServiceUnitsRequest{
		PageSize:  &pageSize,
		PageToken: page1.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, page2.GetServiceUnits(), 1)
	assert.NotEqual(t,
		page1.GetServiceUnits()[0].GetMetadata().GetName(),
		page2.GetServiceUnits()[0].GetMetadata().GetName(),
	)
}

func TestServiceUnitService_DeleteServiceUnit(t *testing.T) {
	c := serviceUnitClient(t)
	ctx := context.Background()

	resp, err := c.CreateServiceUnit(ctx, &buildpb.CreateServiceUnitRequest{Spec: newStaticServiceUnitSpec()})
	require.NoError(t, err)
	name := resp.GetServiceUnit().GetMetadata().GetName()

	delResp, err := c.DeleteServiceUnit(ctx, &buildpb.DeleteServiceUnitRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetServiceUnit(ctx, &buildpb.GetServiceUnitRequest{Name: name})
	assert.Error(t, err, "expected the deleted service unit to be gone")
}

func TestServiceUnitService_WatchServiceUnit(t *testing.T) {
	c := serviceUnitClient(t)
	created := createTestServiceUnit(t, c, newStaticServiceUnitSpec())
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchServiceUnit(ctx, &buildpb.WatchServiceUnitRequest{Name: name})
	require.NoError(t, err)

	// Service units stream indefinitely (no terminal phase) — drive a
	// PENDING -> READY transition directly against the cluster, same as
	// the other watch tests, and cancel once observed. PatchServiceUnit
	// only ever writes .spec, so status has to go through k8sClient
	// directly.
	go func() {
		time.Sleep(200 * time.Millisecond)

		cr := &environmentsv1alpha1.ServiceUnit{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := serviceUnitStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Phase = &commonv1.ServiceUnitDeploymentPhase{
			Phase: commonv1.ServiceUnitDeploymentPhase_SERVICE_UNIT_DEPLOYMENT_PHASE_READY,
		}
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
		phase := event.GetServiceUnit().GetStatus().GetPhase().GetPhase()
		if phase == commonv1.ServiceUnitDeploymentPhase_SERVICE_UNIT_DEPLOYMENT_PHASE_READY {
			sawReady = true
			cancel()
			break
		}
	}
	assert.True(t, sawReady, "expected WatchServiceUnit to deliver the Ready transition")
}
