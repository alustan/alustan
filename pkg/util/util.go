
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
    podName := os.Getenv("POD_NAME")
    podNamespace := os.Getenv("POD_NAMESPACE")
    if podName == "" || podNamespace == "" {
        fmt.Printf("POD_NAME or POD_NAMESPACE environment variables are not set. POD_NAME: %s, POD_NAMESPACE: %s\n", podName, podNamespace)
    }
    return fmt.Sprintf("%s_%s", podNamespace, podName)
}

