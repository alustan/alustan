package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/alustan/alustan/pkg/util"
	"github.com/alustan/alustan/pkg/application/controller"
	"github.com/alustan/alustan/api/app/v1alpha1"
)

// Variables to be set by ldflags
var (
	version string
	commit  string
	date    string
	builtBy string
)

func init() {
	// Register the custom resource types with the global scheme
	utilruntime.Must(v1alpha1.AddToScheme(runtime.NewScheme()))
}


func main() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
	fmt.Printf("Built by: %s\n", builtBy)

	
	_, appSyncInterval := util.GetSyncIntervals()
	log.Printf("Sync interval is set to %v", appSyncInterval)

	// Create a stop channel
	stopCh := make(chan struct{})

	// Create a controller
	ctrl := controller.NewInClusterController(appSyncInterval)

	// Start the reconciliation loop 
	 ctrl.RunLeader(stopCh)

	// Handle shutdown signals to stop the reconciliation loop gracefully
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Println("Received shutdown signal, stopping reconciliation loop...")
		close(stopCh)
		time.Sleep(1 * time.Second) // Give some time for the loop to stop
		os.Exit(0)
	}()

	// Block main goroutine until stopCh is closed
	<-stopCh
}
