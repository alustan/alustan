
package util

import (
	"fmt"
	"os"
)


// containsString checks if a string is present in a slice of strings
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// removeString removes a string from a slice of strings
func RemoveString(slice []string, str string) []string {
	for i, item := range slice {
		if item == str {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}



func GetUniqueID() string {
    podName := os.Getenv("HOSTNAME")
    
    if podName == "" {
        fmt.Printf("HOSTNAME environment variable is not set. POD_NAME: %s", podName)
    }
    return podName
}

