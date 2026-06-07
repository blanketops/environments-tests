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

import (
	"testing"
)

// GitRepositoryService gRPC tests
// APIVersion: sources.blanketops.dev/v1alpha1
// Proto:      blanketops/sources/v1alpha1/gitrepository.proto
// Service:    GitRepositoryService
//
// GitRepository controller reconciles a Crossplane GitHub webhook provider
// resource as an owned child — registering the webhook on GitHub via the API.

func TestGitRepositoryService_CreateGitRepository(t *testing.T) {
	t.Skip("wire: GitRepositoryService.CreateGitRepository — real gRPC server required")
	// d := testdata.NewGitRepositoryData("for-kaniko-app")
	// resp, err := client.CreateGitRepository(ctx, &v1alpha1.CreateGitRepositoryRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "ntlaletsi70", resp.GitRepository.Spec.Contract.Repository.Owner)
	// assert.Equal(t, "github", resp.GitRepository.Spec.Contract.Provider)
}

func TestGitRepositoryService_GetGitRepository(t *testing.T) {
	t.Skip("wire: GitRepositoryService.GetGitRepository — real gRPC server required")
}

func TestGitRepositoryService_ListGitRepositories(t *testing.T) {
	t.Skip("wire: GitRepositoryService.ListGitRepositories — filter by provider")
}

func TestGitRepositoryService_DeleteGitRepository(t *testing.T) {
	t.Skip("wire: GitRepositoryService.DeleteGitRepository — Crossplane webhook deregistered on delete")
}

func TestGitRepositoryService_WatchGitRepository(t *testing.T) {
	t.Skip("wire: GitRepositoryService.WatchGitRepository — streaming phase transitions")
	// Pending → Ready when webhook registered and hookUrl reachable
}

func TestGitRepositoryService_PushOnly(t *testing.T) {
	t.Skip("wire: GitRepositoryService.CreateGitRepository — push events only")
	// d := testdata.NewPushOnlyGitRepositoryData("for-kaniko-app-push-only")
}

func TestGitRepositoryService_CustomHookURL(t *testing.T) {
	t.Skip("wire: GitRepositoryService.CreateGitRepository — production ingress hookUrl")
	// d := testdata.NewGitRepositoryDataWithHookURL("for-kaniko-app", "https://events.blanketops.dev/webhooks/github")
}
