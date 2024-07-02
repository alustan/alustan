package util

import (
	"log"
	"os"
	"time"
)

const (
	defaultSyncInterval    = 360 * time.Minute // Default sync interval 6 hrs
	
)

// GetSyncInterval retrieves the sync interval  from the environment variable
// or returns the default value.
func GetSyncInterval() time.Duration {
	return getEnvSyncInterval("SYNC_INTERVAL", defaultSyncInterval)
}

// getEnvSyncInterval is a helper function that retrieves the sync interval from the specified
// environment variable or returns the provided default value.
func getEnvSyncInterval(envVar string, defaultInterval time.Duration) time.Duration {
	syncIntervalStr := os.Getenv(envVar)
	if syncIntervalStr == "" {
		log.Printf("%s not set, using default value: %v", envVar, defaultInterval)
		return defaultInterval
	}

	syncInterval, err := time.ParseDuration(syncIntervalStr)
	if err != nil {
		log.Printf("Invalid %s format, using default value: %v. Error: %v", envVar, defaultInterval, err)
		return defaultInterval
	}

	log.Printf("Using %s from environment: %v", envVar, syncInterval)
	return syncInterval
}