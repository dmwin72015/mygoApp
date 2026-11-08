package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mygo-app/internal/config"
	"mygo-app/internal/data/mysql"
	"mygo-app/internal/data/postgres"
	"mygo-app/internal/domain"
	"mygo-app/internal/server"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	var (
		configFlag = flag.String("config", "configs/server.yaml", "Path to YAML configuration file")
		portFlag   = flag.Int("port", 0, "HTTP port to listen on (overrides config file)")
		driverFlag = flag.String("driver", "", "Database driver: 'postgres', 'mysql', or 'sqlite' (overrides config file)")
		dsnFlag    = flag.String("dsn", "", "Database connection string or SQLite file path (overrides config file)")
	)
	flag.StringVar(configFlag, "c", "configs/server.yaml", "Short alias for -config")
	flag.Parse()

	// Load configuration from YAML file and environment variables
	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// CLI flags take precedence over YAML config if specified
	if *portFlag != 0 {
		cfg.Server.Port = *portFlag
	}
	if *driverFlag != "" {
		cfg.Database.Driver = strings.ToLower(strings.TrimSpace(*driverFlag))
	}
	if *dsnFlag != "" {
		cfg.Database.DSN = *dsnFlag
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driver := cfg.Database.Driver
	dsn := cfg.Database.DSN

	maskedDSN := postgres.SanitizeDSN(dsn)
	if driver == "mysql" {
		maskedDSN = mysql.SanitizeDSN(dsn)
	}
	log.Printf("Connecting to %s database at %s ...", driver, maskedDSN)
	var repo domain.Repo
	switch driver {
	case "postgres", "postgresql", "pgx":
		repo, err = postgres.Open(ctx, dsn)
	case "mysql":
		repo, err = mysql.Open(ctx, dsn)
	default:
		log.Fatalf("Unsupported server database driver: %s (supported: postgres, mysql)", driver)
	}

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer repo.Close()

	srv := server.New(repo)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown handling
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("==================================================")
		log.Printf("启明培训 · 业务服务端 (HTTP API Server) 已启动")
		log.Printf("配置文件: %s", *configFlag)
		log.Printf("数据库引擎: %s (%s)", driver, repo.Source())
		log.Printf("监听地址: http://127.0.0.1%s", addr)
		log.Printf("默认管理员账号: admin / admin123")
		log.Printf("客户端启动命令: SCHOOL_DRIVER=remote SCHOOL_API=http://127.0.0.1%s go run .", addr)
		log.Printf("==================================================")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("Shutting down HTTP server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	_ = srv.Shutdown(shutdownCtx)
	log.Println("Server stopped.")
}
