package main

import (
	"fmt"
	"log"

	"github.com/usual2970/later/configs"
)

func main() {
	cfg, err := configs.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("✅ Configuration Loaded Successfully")
	fmt.Println("==================================")
	fmt.Printf("Server:\n")
	fmt.Printf("  Host: %s\n", cfg.Server.Host)
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Printf("  Address: %s\n", cfg.Server.Address())

	fmt.Printf("\nDatabase:\n")
	fmt.Printf("  URL: %s\n", cfg.Database.URL)
	fmt.Printf("  Max Connections: %d\n", cfg.Database.MaxConnections)

	fmt.Printf("\nScheduler:\n")
	fmt.Printf("  High Priority Interval: %v\n", cfg.Scheduler.HighPriorityInterval)
	fmt.Printf("  Normal Priority Interval: %v\n", cfg.Scheduler.NormalPriorityInterval)
	fmt.Printf("  Cleanup Interval: %v\n", cfg.Scheduler.CleanupInterval)

	fmt.Printf("\nWorker:\n")
	fmt.Printf("  Pool Size: %d\n", cfg.Worker.PoolSize)

	fmt.Printf("\nCallback:\n")
	fmt.Printf("  Secret: %s\n", maskSecret(cfg.Callback.Secret))
	fmt.Printf("  Default Timeout: %v\n", cfg.Callback.DefaultTimeout)
	fmt.Printf("  Default Max Retries: %d\n", cfg.Callback.DefaultMaxRetries)

	fmt.Printf("\nLogging:\n")
	fmt.Printf("  Level: %s\n", cfg.Log.Level)
	fmt.Printf("  Format: %s\n", cfg.Log.Format)

	fmt.Println("\n==================================")
	fmt.Println("✅ All configurations are valid!")
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}
