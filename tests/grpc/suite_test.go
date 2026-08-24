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
// BuildService is wired as the reference implementation: an in-process
// gRPC server (TestMain below) backed by the real controller-runtime
// client against whatever cluster KUBECONFIG resolves to — the same
// real-cluster connection tests/fake uses, just with a gRPC service
// implementation as the seam instead of a fake domain. No fakes, no
// mocks: CreateBuild/GetBuild/etc. do real K8s CRUD, PatchBuild applies
// a real RFC 7396 merge patch, WatchBuild opens a real k8s watch.
//
// The rest of the services below are still stubbed with t.Skip() —
// follow the BuildService wiring in build_server_test.go and
// build_grpc_test.go as the pattern to extend to each.
//
// Covered services:
//   BuildService          blanketops/environments/v1alpha1/build.proto  [wired]
//   DeploymentService     blanketops/environments/v1/deployment.proto
//   PackageService        blanketops/environments/v1/package.proto
//   ServiceUnitService    blanketops/environments/v1alpha1/serviceunit.proto
//   GitHubEventService    blanketops/events/v1alpha1/githubevent.proto
//   GitRepositoryService  blanketops/sources/v1alpha1/gitrepository.proto
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
