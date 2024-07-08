package util

import (
	"fmt"
)




func FormatEnvVars(envVars map[string]string) []string {
	formattedVars := make([]string, 0, len(envVars))
	for key, value := range envVars {
		formattedVars = append(formattedVars, fmt.Sprintf("%s=%s", key, value))
	}
	return formattedVars
}


func ExtractEnvVars(variables map[string]string) map[string]string {
	if variables == nil {
		return nil
	}
	envVars := make(map[string]string)
	for key, value := range variables {
		envVars[key] = value
	}
	
	return envVars
}

