package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"mygo-app/internal/config"
	"mygo-app/internal/data/mysql"
	"mygo-app/internal/data/postgres"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const usage = `启明培训 · 数据库迁移工具 (Database Migration CLI)

用法 (Usage):
  go run ./cmd/migrate [命令/Command] [参数/Flags]

核心命令 (Commands):
  up                   升级到最新版本 (Apply all pending migrations)
  down                 回滚上一个版本 (Roll back the last applied migration)
  redo                 重跑最新版本 (Roll back latest and run it again)
  reset                重置所有版本 (Roll back all migrations to empty state)
  status               查看迁移状态 (Show migration status table)
  version              查看当前数据库版本号 (Print current database version)
  create <name>        创建新的迁移文件 (Create a new sequential SQL migration file)

常用参数 (Flags):
  -config string       配置文件路径 (默认 "configs/server.yaml")
  -driver string       数据库驱动: "postgres" 或 "mysql" (默认从配置文件读取)
  -dsn string          数据库连接串 (覆写配置文件中的 DSN)
  -dir string          迁移脚本所在目录 (默认根据驱动自动选择: internal/data/postgres/migrations 或 internal/data/mysql/migrations)

示例 (Examples):
  go run ./cmd/migrate status
  go run ./cmd/migrate up
  go run ./cmd/migrate down
  go run ./cmd/migrate redo
  go run ./cmd/migrate create add_student_gender
  go run ./cmd/migrate -driver mysql -dsn "root:password@tcp(127.0.0.1:3306)/school" up
  go run ./cmd/migrate -driver postgres -dsn "postgres://user:pwd@127.0.0.1:5432/school?sslmode=disable" up
`

func main() {
	configFlag := flag.String("config", "configs/server.yaml", "Path to YAML configuration file")
	driverFlag := flag.String("driver", "", "Database driver: 'postgres' or 'mysql' (overrides config file)")
	dsnFlag := flag.String("dsn", "", "Database connection string (overrides config file)")
	dirFlag := flag.String("dir", "", "Path to migrations directory")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	command := strings.ToLower(strings.TrimSpace(args[0]))

	// Load configuration first to resolve defaults
	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Printf("提示: 读取配置文件 %s 失败 (%v)，将使用内置默认配置", *configFlag, err)
		cfg = config.Default()
	}

	driver := cfg.Database.Driver
	if *driverFlag != "" {
		driver = strings.ToLower(strings.TrimSpace(*driverFlag))
	}
	if driver == "" {
		driver = "postgres"
	}

	// Resolve default migrations dir based on driver
	targetDir := *dirFlag
	if targetDir == "" {
		if driver == "mysql" {
			targetDir = "internal/data/mysql/migrations"
		} else {
			targetDir = "internal/data/postgres/migrations"
		}
	}

	// Handle 'create' command (does not require DB connection)
	if command == "create" {
		if len(args) < 2 {
			log.Fatalf("错误: 请提供迁移名称，例如: go run ./cmd/migrate create add_user_avatar")
		}
		name := strings.TrimSpace(args[1])
		goose.SetSequential(true)
		if err := goose.Create(nil, targetDir, name, "sql"); err != nil {
			log.Fatalf("创建迁移文件失败: %v", err)
		}
		log.Printf("成功在 %s 目录下创建新迁移文件: %s", targetDir, name)
		return
	}

	dsn := cfg.Database.DSN
	if *dsnFlag != "" {
		dsn = *dsnFlag
	}

	if dsn == "" {
		log.Fatalf("错误: 未配置数据库连接串 (DSN)，请通过 -dsn 参数或 configs/server.yaml 配置")
	}

	ctx := context.Background()

	var (
		db        *sql.DB
		maskedDSN string
	)

	switch driver {
	case "postgres", "postgresql", "pgx":
		maskedDSN = postgres.SanitizeDSN(dsn)
		log.Printf("[MIGRATE] 正在连接 PostgreSQL 数据库: %s ...", maskedDSN)
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("连接 PostgreSQL 失败: %v", err)
		}
		defer db.Close()

		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("数据库 Ping 失败 (%s): %v", maskedDSN, err)
		}

		if err := postgres.Migrate(ctx, db, command); err != nil {
			log.Fatalf("执行 PostgreSQL 迁移 [%s] 失败: %v", command, err)
		}

	case "mysql":
		maskedDSN = mysql.SanitizeDSN(dsn)
		normDSN := mysql.NormalizeDSN(dsn)
		log.Printf("[MIGRATE] 正在连接 MySQL 数据库: %s ...", maskedDSN)
		db, err = sql.Open("mysql", normDSN)
		if err != nil {
			log.Fatalf("连接 MySQL 失败: %v", err)
		}
		defer db.Close()

		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("数据库 Ping 失败 (%s): %v", maskedDSN, err)
		}

		if err := mysql.Migrate(ctx, db, command); err != nil {
			log.Fatalf("执行 MySQL 迁移 [%s] 失败: %v", command, err)
		}

	default:
		log.Fatalf("不支持的迁移数据库驱动: %s (支持: postgres, mysql)", driver)
	}

	log.Printf("[MIGRATE] 迁移动作 [%s] 执行完毕。", command)
}
