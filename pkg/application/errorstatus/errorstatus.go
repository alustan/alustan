package errorstatus

import (
	
	"fmt"
	
	
	"github.com/alustan/alustan/api/app/v1alpha1"
	"go.uber.org/zap"

)



func ErrorResponse(logger *zap.SugaredLogger,action string, err error) v1alpha1.AppStatus {
	logger.Infof("Error %s: %v", action, err)
	return v1alpha1.AppStatus{
		State:   "Error",
		Message: fmt.Sprintf("Error %s: %v", action, err),
	}
}