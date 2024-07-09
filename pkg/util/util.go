
package util

import (
	
	"fmt"
	
	"github.com/google/uuid"
	"github.com/alustan/api/v1alpha1"
	"go.uber.org/zap"

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

func ErrorResponse(logger *zap.SugaredLogger,action string, err error) v1alpha1.TerraformStatus {
	logger.Infof("Error %s: %v", action, err)
	return v1alpha1.TerraformStatus{
		State:   "Error",
		Message: fmt.Sprintf("Error %s: %v", action, err),
	}
}

func GetUniqueID() string {
	return uuid.New().String()
}



