package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/alustan/alustan/pkg/util"
	"github.com/alustan/alustan/pkg/infrastructure/controller"
	"github.com/alustan/alustan/api/infrastructure/v1alpha1"
	"go.uber.org/zap"
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

	// Initialize logger with debug level
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	defer logger.Sync() // Ensure logger is flushed on shutdown
	sugar := logger.Sugar()

	infraSyncInterval, _ := util.GetSyncIntervals()
	sugar.Infof("Sync interval is set to %v", infraSyncInterval)

	// Create a stop channel
	stopCh := make(chan struct{})

	// Create a controller and pass the logger
	ctrl := controller.NewInClusterController(infraSyncInterval, sugar)

	// Start the reconciliation loop
	go func() {
		ctrl.RunLeader(stopCh)
	}()

	// Handle shutdown signals to stop the reconciliation loop gracefully
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		sugar.Info("Received shutdown signal, stopping reconciliation loop...")
		close(stopCh)
		time.Sleep(1 * time.Second) // Give some time for the loop to stop
		logger.Sync()
		os.Exit(0)
	}()

	// Block main goroutine until stopCh is closed
	<-stopCh
}
