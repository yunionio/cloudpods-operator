package controller

import "testing"

func TestGenerateAllPolicies(t *testing.T) {
	policies := generateAllPolicies()
	for _, p := range policies {
		t.Logf("name %s description %s scope %s policy %s", p.name, p.description, p.scope, p.policy)
	}
	t.Logf("total: %d", len(policies))
}
