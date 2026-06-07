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
// All tests in this package are stubbed with t.Skip().
// They compile and run green until the gRPC server is wired.
//
// To wire:
//   1. Set BLANKETOPS_GRPC_ADDR to the server address
//   2. Implement grpcAddr() and per-service client constructors
//   3. Remove t.Skip() from each test
//   4. Wire the testdata fixtures to the proto request types
//
// Covered services:
//   BuildService          blanketops/environments/v1alpha1/build.proto
//   BuildTriggerService   blanketops/environments/v1/buildtrigger.proto
//   DeploymentService     blanketops/environments/v1/deployment.proto
//   PackageService        blanketops/environments/v1/package.proto
//   ServiceUnitService    blanketops/environments/v1alpha1/serviceunit.proto
//   GitHubEventService    blanketops/events/v1alpha1/githubevent.proto
//   GitRepositoryService  blanketops/sources/v1alpha1/gitrepository.proto
//   RouteService          blanketops/networks/v1alpha1/route.proto
//   DomainService         blanketops/networks/v1alpha1/domain.proto
