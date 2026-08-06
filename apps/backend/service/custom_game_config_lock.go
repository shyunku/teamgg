package service

import "sync"

type customGameConfigurationLock struct {
	mutex      sync.RWMutex
	stateMutex sync.RWMutex
	optimizing bool
}

var customGameConfigurationLocks sync.Map

func getCustomGameConfigurationLock(configId string) *customGameConfigurationLock {
	lock, _ := customGameConfigurationLocks.LoadOrStore(configId, &customGameConfigurationLock{})
	return lock.(*customGameConfigurationLock)
}

func TryBeginCustomGameConfigurationMutation(configId string) bool {
	return getCustomGameConfigurationLock(configId).mutex.TryRLock()
}

func EndCustomGameConfigurationMutation(configId string) {
	getCustomGameConfigurationLock(configId).mutex.RUnlock()
}

func TryLockCustomGameConfigurationForOptimization(configId string) bool {
	lock := getCustomGameConfigurationLock(configId)
	if !lock.mutex.TryLock() {
		return false
	}
	lock.stateMutex.Lock()
	lock.optimizing = true
	lock.stateMutex.Unlock()
	return true
}

func UnlockCustomGameConfigurationForOptimization(configId string) {
	lock := getCustomGameConfigurationLock(configId)
	lock.stateMutex.Lock()
	lock.optimizing = false
	lock.stateMutex.Unlock()
	lock.mutex.Unlock()
}

func IsCustomGameConfigurationOptimizing(configId string) bool {
	lock := getCustomGameConfigurationLock(configId)
	lock.stateMutex.RLock()
	defer lock.stateMutex.RUnlock()
	return lock.optimizing
}
