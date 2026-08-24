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

// PackageService gRPC tests
// APIVersion: environments.blanketops.dev/v1
// Proto:      blanketops/environments/v1/package.proto
// Service:    PackageService
//
// Each test calls the real PackageService over a real gRPC connection
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

func newPackageSpec() *deploymentpb.PackageSpec {
	return &deploymentpb.PackageSpec{
		Enabled:     true,
		Name:        "for-kaniko-app",
		Version:     "v1.2.3",
		Description: "for-kaniko-app deployment package",
		Maintainers: []*deploymentpb.Maintainer{
			{Name: "Neo", Email: "ntlaletsi86@gmail.com"},
		},
		Repository: &deploymentpb.PackageRepository{
			Url:               "git@github.com:ntlaletsi70/for-kaniko-app-package.git",
			CredentialsSecret: "git-ssh-credentials",
		},
		DiffEnabled: true,
		StateRepository: &deploymentpb.StateRepository{
			Url:         "git@github.com:ntlaletsi70/for-kaniko-app-state.git",
			Ref:         "main",
			CloneSecret: "git-ssh-credentials",
			Strategy:    "kustomization",
			Path:        "./clusters/dev",
		},
	}
}

// createTestPackage creates a Package via the real gRPC service and
// registers its cleanup, returning the created proto Package.
func createTestPackage(t *testing.T, c deploymentpb.PackageServiceClient) *deploymentpb.Package {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreatePackage(ctx, &deploymentpb.CreatePackageRequest{Spec: newPackageSpec()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetPackage())

	name := resp.GetPackage().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeletePackage(context.Background(), &deploymentpb.DeletePackageRequest{Name: name})
	})

	return resp.GetPackage()
}

func TestPackageService_CreatePackage(t *testing.T) {
	c := packageClient(t)

	pkg := createTestPackage(t, c)

	assert.NotEmpty(t, pkg.GetMetadata().GetName())
	assert.NotEmpty(t, pkg.GetMetadata().GetId())
	assert.Equal(t, "v1.2.3", pkg.GetSpec().GetVersion())
	require.Len(t, pkg.GetSpec().GetMaintainers(), 1)
	assert.Equal(t, "Neo", pkg.GetSpec().GetMaintainers()[0].GetName())
}

func TestPackageService_GetPackage(t *testing.T) {
	c := packageClient(t)
	created := createTestPackage(t, c)

	resp, err := c.GetPackage(context.Background(), &deploymentpb.GetPackageRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetPackage())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetPackage().GetMetadata().GetName())
	assert.Equal(t, created.GetSpec().GetVersion(), resp.GetPackage().GetSpec().GetVersion())
}

func TestPackageService_UpdatePackage(t *testing.T) {
	c := packageClient(t)
	ctx := context.Background()
	created := createTestPackage(t, c)

	newSpec := newPackageSpec()
	newSpec.Version = "v2.0.0"

	resp, err := c.UpdatePackage(ctx, &deploymentpb.UpdatePackageRequest{
		Package: &deploymentpb.Package{
			Metadata: &commonv1.Metadata{Name: created.GetMetadata().GetName()},
			Spec:     newSpec,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", resp.GetPackage().GetSpec().GetVersion())

	getResp, err := c.GetPackage(ctx, &deploymentpb.GetPackageRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", getResp.GetPackage().GetSpec().GetVersion())
}

func TestPackageService_PatchPackage(t *testing.T) {
	c := packageClient(t)
	ctx := context.Background()
	created := createTestPackage(t, c)

	patch := `{"spec":{"version":"v1.3.0","diff_enabled":false}}`
	resp, err := c.PatchPackage(ctx, &deploymentpb.PatchPackageRequest{
		Name:  created.GetMetadata().GetName(),
		Patch: patch,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetPackage())

	assert.Equal(t, "v1.3.0", resp.GetPackage().GetSpec().GetVersion())
	assert.False(t, resp.GetPackage().GetSpec().GetDiffEnabled())
	// Fields outside the patch survive untouched.
	assert.Equal(t, "for-kaniko-app", resp.GetPackage().GetSpec().GetName())
}

func TestPackageService_ListPackages(t *testing.T) {
	c := packageClient(t)
	ctx := context.Background()
	created := createTestPackage(t, c)

	resp, err := c.ListPackages(ctx, &deploymentpb.ListPackagesRequest{})
	require.NoError(t, err)

	var found bool
	for _, p := range resp.GetPackages() {
		if p.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListPackages to include the created package")

	enabled := true
	filtered, err := c.ListPackages(ctx, &deploymentpb.ListPackagesRequest{Enabled: &enabled})
	require.NoError(t, err)
	for _, p := range filtered.GetPackages() {
		assert.True(t, p.GetSpec().GetEnabled())
	}
}

func TestPackageService_ListPackages_Pagination(t *testing.T) {
	c := packageClient(t)

	for range 3 {
		createTestPackage(t, c)
	}

	pageSize := int32(1)
	page1, err := c.ListPackages(context.Background(), &deploymentpb.ListPackagesRequest{PageSize: &pageSize})
	require.NoError(t, err)
	assert.Len(t, page1.GetPackages(), 1)
	require.NotNil(t, page1.NextPageToken)
	assert.NotEmpty(t, page1.GetNextPageToken())

	page2, err := c.ListPackages(context.Background(), &deploymentpb.ListPackagesRequest{
		PageSize:  &pageSize,
		PageToken: page1.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, page2.GetPackages(), 1)
	assert.NotEqual(t,
		page1.GetPackages()[0].GetMetadata().GetName(),
		page2.GetPackages()[0].GetMetadata().GetName(),
	)
}

func TestPackageService_DeletePackage(t *testing.T) {
	c := packageClient(t)
	ctx := context.Background()

	resp, err := c.CreatePackage(ctx, &deploymentpb.CreatePackageRequest{Spec: newPackageSpec()})
	require.NoError(t, err)
	name := resp.GetPackage().GetMetadata().GetName()

	delResp, err := c.DeletePackage(ctx, &deploymentpb.DeletePackageRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetPackage(ctx, &deploymentpb.GetPackageRequest{Name: name})
	assert.Error(t, err, "expected the deleted package to be gone")
}

func TestPackageService_WatchPackage(t *testing.T) {
	c := packageClient(t)
	created := createTestPackage(t, c)
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchPackage(ctx, &deploymentpb.WatchPackageRequest{Name: name})
	require.NoError(t, err)

	// Packages stream indefinitely (no terminal phase) — drive a
	// PENDING -> READY transition directly against the cluster, same as
	// the Build/Deployment watch tests, and cancel once observed.
	// PatchPackage only ever writes .spec (see packageServer.PatchPackage),
	// so status has to go through k8sClient directly.
	go func() {
		time.Sleep(200 * time.Millisecond)

		cr := &environmentsv1alpha1.Package{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := packageStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Phase = &commonv1.PackagePhase{Phase: commonv1.PackagePhase_PACKAGE_PHASE_READY}
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
		if event.GetPackage().GetStatus().GetPhase().GetPhase() == commonv1.PackagePhase_PACKAGE_PHASE_READY {
			sawReady = true
			cancel()
			break
		}
	}
	assert.True(t, sawReady, "expected WatchPackage to deliver the Ready transition")
}

func TestPackageService_DisabledPackage(t *testing.T) {
	c := packageClient(t)

	spec := newPackageSpec()
	spec.Enabled = false

	resp, err := c.CreatePackage(context.Background(), &deploymentpb.CreatePackageRequest{Spec: spec})
	require.NoError(t, err)
	name := resp.GetPackage().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeletePackage(context.Background(), &deploymentpb.DeletePackageRequest{Name: name})
	})

	assert.False(t, resp.GetPackage().GetSpec().GetEnabled())
}
