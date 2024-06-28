package util


func ExtractEnvVars(variables map[string]string) map[string]string {
	envVars := make(map[string]string)
	for key, value := range variables {
		envVars[key] = value
	}
	
	return envVars
}

