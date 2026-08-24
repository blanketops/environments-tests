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

// gRPC test suite
//
// BuildService, DeploymentService, PackageService, ServiceUnitService,
// GitHubEventService, and GitRepositoryService are wired as the
// reference implementations: an in-process gRPC server (TestMain below)
// backed by the real controller-runtime client against whatever cluster
// KUBECONFIG resolves to — the same real-cluster connection tests/fake
// uses, just with a gRPC service implementation as the seam instead of
// a fake domain. No fakes, no mocks: Create/Get/etc. do real K8s CRUD,
// Patch applies a real RFC 7396 merge patch, Watch opens a real k8s
// watch.
//
// The rest of the services below are still stubbed with t.Skip() —
// follow build_server_test.go/build_grpc_test.go,
// deployment_server_test.go/deployment_grpc_test.go,
// package_server_test.go/package_grpc_test.go,
// serviceunit_server_test.go/serviceunit_grpc_test.go,
// githubevent_server_test.go/githubevent_grpc_test.go, and
// gitrepository_server_test.go/gitrepository_grpc_test.go as the
// pattern to extend to each.
//
// Covered services:
//   BuildService          blanketops/environments/v1alpha1/build.proto       [wired]
//   DeploymentService     blanketops/environments/v1/deployment.proto        [wired]
//   PackageService        blanketops/environments/v1/package.proto           [wired]
//   ServiceUnitService    blanketops/environments/v1alpha1/serviceunit.proto [wired]
//   GitHubEventService    blanketops/events/v1alpha1/githubevent.proto       [wired]
//   GitRepositoryService  blanketops/sources/v1alpha1/gitrepository.proto    [wired]
//   RouteService          blanketops/networks/v1alpha1/route.proto
//   DomainService         blanketops/networks/v1alpha1/domain.proto

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	sourcesv1alpha1 "github.com/blanketops/environments-api/api/sources/v1alpha1"
	githubeventpb "github.com/blanketops/environments-contract/blanketops/events/v1alpha1"
	gitrepositorypb "github.com/blanketops/environments-contract/blanketops/sources/v1alpha1"

	// deploymentpb covers every service generated into the shared
	// blanketops/environments/v1 package — currently DeploymentService
	// and PackageService.
	deploymentpb "github.com/blanketops/environments-contract/blanketops/environments/v1"
	// buildpb covers every service generated into the shared
	// blanketops/environments/v1alpha1 package — currently BuildService.
	buildpb "github.com/blanketops/environments-contract/blanketops/environments/v1alpha1"
)

// testNamespace is the fixed namespace every gRPC service in this suite
// operates in — matches tests/fake's convention. Real requests, real
// cluster, one namespace to keep the reference implementation simple.
const testNamespace = "default"

var (
	k8sClient client.WithWatch
	grpcConn  *grpc.ClientConn
)

// This suite runs against a real, already-running cluster (CRDs installed
// via environments-install) — it does not stand up its own control plane.
// The gRPC server started here is real too, just hosted in-process over a
// bufconn pipe instead of a separate binary.
func TestMain(m *testing.M) {
	if err := environmentsv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintln(os.Stderr, "add to scheme:", err)
		os.Exit(1)
	}
	if err := eventsv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintln(os.Stderr, "add to scheme:", err)
		os.Exit(1)
	}
	if err := sourcesv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintln(os.Stderr, "add to scheme:", err)
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "get kubeconfig:", err)
		os.Exit(1)
	}

	k8sClient, err = client.NewWithWatch(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Fprintln(os.Stderr, "new k8s client:", err)
		os.Exit(1)
	}

	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	buildpb.RegisterBuildServiceServer(grpcServer, &buildServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	deploymentpb.RegisterDeploymentServiceServer(grpcServer, &deploymentServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	deploymentpb.RegisterPackageServiceServer(grpcServer, &packageServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	buildpb.RegisterServiceUnitServiceServer(grpcServer, &serviceUnitServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	githubeventpb.RegisterGitHubEventServiceServer(grpcServer, &githubEventServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	gitrepositorypb.RegisterGitRepositoryServiceServer(grpcServer, &gitRepositoryServer{
		client:    k8sClient,
		namespace: testNamespace,
	})
	go func() {
		_ = grpcServer.Serve(lis)
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial in-process server:", err)
		os.Exit(1)
	}
	grpcConn = conn

	code := m.Run()

	_ = conn.Close()
	grpcServer.Stop()
	os.Exit(code)
}

func buildClient(t *testing.T) buildpb.BuildServiceClient {
	t.Helper()
	return buildpb.NewBuildServiceClient(grpcConn)
}

func deploymentClient(t *testing.T) deploymentpb.DeploymentServiceClient {
	t.Helper()
	return deploymentpb.NewDeploymentServiceClient(grpcConn)
}

func packageClient(t *testing.T) deploymentpb.PackageServiceClient {
	t.Helper()
	return deploymentpb.NewPackageServiceClient(grpcConn)
}

func serviceUnitClient(t *testing.T) buildpb.ServiceUnitServiceClient {
	t.Helper()
	return buildpb.NewServiceUnitServiceClient(grpcConn)
}

func githubEventClient(t *testing.T) githubeventpb.GitHubEventServiceClient {
	t.Helper()
	return githubeventpb.NewGitHubEventServiceClient(grpcConn)
}

func gitRepositoryClient(t *testing.T) gitrepositorypb.GitRepositoryServiceClient {
	t.Helper()
	return gitrepositorypb.NewGitRepositoryServiceClient(grpcConn)
}
