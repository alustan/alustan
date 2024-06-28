package main

import (
	"log"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/app-controller/controller" 
	
)

// Variables to be set by ldflags
var (
	version  string
	commit   string
	date     string
	builtBy  string
)

func main() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
	fmt.Printf("Built by: %s\n", builtBy)
	
	r := gin.Default()
	
	_, appSyncInterval := util.GetSyncIntervals()
	log.Printf("Sync interval is set to %v", appSyncInterval)

	ctrl := controller.NewInClusterController(appSyncInterval)

	// Start the reconciliation loop in a separate goroutine
	go ctrl.Reconcile()

	r.POST("/sync", ctrl.ServeHTTP)

	log.Println("Starting server on port 8081...")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}


