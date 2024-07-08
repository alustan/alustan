package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/controller"
)

// Variables to be set by ldflags
var (
	version string
	commit  string
	date    string
	builtBy string
)

func main() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
	fmt.Printf("Built by: %s\n", builtBy)

	syncInterval := util.GetSyncInterval()
	log.Printf("Sync interval is set to %v", syncInterval)

	// Create a stop channel
	stopCh := make(chan struct{})

	// Create a controller
	ctrl := controller.NewInClusterController(syncInterval)

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
