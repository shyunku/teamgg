package service

import "testing"

func TestCustomGameConfigurationOptimizationLockBlocksMutations(t *testing.T) {
	const configId = "lock-test-optimization"
	if !TryLockCustomGameConfigurationForOptimization(configId) {
		t.Fatal("optimization lock should be acquired")
	}
	if !IsCustomGameConfigurationOptimizing(configId) {
		t.Fatal("configuration should report optimizing")
	}
	if TryBeginCustomGameConfigurationMutation(configId) {
		EndCustomGameConfigurationMutation(configId)
		t.Fatal("mutation must be rejected while optimizing")
	}
	UnlockCustomGameConfigurationForOptimization(configId)
	if IsCustomGameConfigurationOptimizing(configId) {
		t.Fatal("configuration should be unlocked")
	}
	if !TryBeginCustomGameConfigurationMutation(configId) {
		t.Fatal("mutation should be allowed after optimization")
	}
	EndCustomGameConfigurationMutation(configId)
}

func TestCustomGameConfigurationMutationBlocksOptimization(t *testing.T) {
	const configId = "lock-test-mutation"
	if !TryBeginCustomGameConfigurationMutation(configId) {
		t.Fatal("mutation lock should be acquired")
	}
	if TryLockCustomGameConfigurationForOptimization(configId) {
		UnlockCustomGameConfigurationForOptimization(configId)
		t.Fatal("optimization must be rejected during a mutation")
	}
	EndCustomGameConfigurationMutation(configId)
	if !TryLockCustomGameConfigurationForOptimization(configId) {
		t.Fatal("optimization should be allowed after mutation")
	}
	UnlockCustomGameConfigurationForOptimization(configId)
}
