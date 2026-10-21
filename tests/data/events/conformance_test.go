/*
Copyright 2026 BlanketOps.

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
package events

import (
	"encoding/json"
	"testing"

	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	githubeventresolve "github.com/blanketops/environments/resolution/githubevent/resolve"
	"k8s.io/apimachinery/pkg/runtime"
)

// Every fixture builder must produce a contract the engine's own resolver
// accepts; see tests/data/environments/conformance_test.go.

func TestFixturesResolveWithTheEngine(t *testing.T) {
	const env = "for-kaniko-app"
	events := map[string]*GitHubEventData{
		"NewGitHubEventData":            NewGitHubEventData("push-1", env),
		"NewPullRequestGitHubEventData": NewPullRequestGitHubEventData("pr-1", env),
		"NewMasterGitHubEventData":      NewMasterGitHubEventData("push-master-1", env),
	}
	for name, d := range events {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(d.Spec.Contract)
			if err != nil {
				t.Fatalf("marshal contract: %v", err)
			}
			cr := &eventsv1alpha1.GitHubEvent{ObjectMeta: d.ObjectMeta,
				Spec: eventsv1alpha1.GitHubEventSpec{Contract: runtime.RawExtension{Raw: raw}}}
			if _, err := githubeventresolve.ResolveGitHubEvent(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}
}
