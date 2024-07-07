package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/alustan/pkg/controller"
    "github.com/alustan/pkg/util"
    "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/tools/leaderelection"
    "k8s.io/client-go/tools/leaderelection/resourcelock"
)

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

    syncInterval := util.GetSyncInterval()
    log.Printf("Sync interval is set to %v", syncInterval)

    // Create a stop channel
    stopCh := make(chan struct{})

    // Create a controller using in-cluster configuration
    ctrl := controller.NewInClusterController(syncInterval)

    // Create leader election configuration
    electionConfig := leaderelection.LeaderElectionConfig{
        Lock: &resourcelock.LeaseLock{
            LeaseMeta: v1.ObjectMeta{
                Namespace: "default",
                Name:      "controller-lock",
            },
            Client: ctrl.Clientset.CoordinationV1(),
            LockConfig: resourcelock.ResourceLockConfig{
                Identity: "controller-leader",
            },
        },
        ReleaseOnCancel: true,
        LeaseDuration:   15 * time.Second,
        RenewDeadline:   10 * time.Second,
        RetryPeriod:     2 * time.Second,
        Callbacks: leaderelection.LeaderCallbacks{
            OnStartedLeading: func(ctx context.Context) {
                log.Println("Started leading...")
                go func() {
                    // Start the reconciliation loop
                    ctrl.Reconcile(stopCh)
                }()
            },
            OnStoppedLeading: func() {
                log.Println("Stopped leading...")
                close(stopCh)
            },
        },
    }

    // Create a leader election runner
    leaderElector, err := leaderelection.NewLeaderElector(electionConfig)
    if err != nil {
        log.Fatalf("Error creating leader elector: %v", err)
    }

    // Start the leader election
    go func() {
        log.Println("Starting leader election...")
        leaderElector.Run(context.Background())
    }()

    // Handle shutdown signals to stop the reconciliation loop gracefully
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-signalChan
        log.Println("Received shutdown signal, stopping reconciliation loop...")
        close(stopCh)
        time.Sleep(1 * time.Second) // Give some time for the leader election to stop
        os.Exit(0)
    }()

    r.POST("/sync", ctrl.ServeHTTP)

    log.Println("Starting server on port 8080...")
    if err := r.Run(":8080"); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
