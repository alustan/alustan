package errorstatus

import (
	
	"fmt"
	
	
	"github.com/alustan/alustan/api/service/v1alpha1"
	"go.uber.org/zap"

)



func ErrorResponse(logger *zap.SugaredLogger,action string, err error) v1alpha1.ServiceStatus {
	logger.Infof("Error %s: %v", action, err)
	return v1alpha1.ServiceStatus{
		State:   "Error",
		Message: fmt.Sprintf("Error %s: %v", action, err),
	}
}