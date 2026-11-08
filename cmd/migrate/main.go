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
	"mygo-app/internal/data/postgres"

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
  -dsn string          数据库连接串 (覆写配置文件中的 DSN)
  -dir string          迁移脚本所在目录 (默认 "internal/data/postgres/migrations")

示例 (Examples):
  go run ./cmd/migrate status
  go run ./cmd/migrate up
  go run ./cmd/migrate down
  go run ./cmd/migrate redo
  go run ./cmd/migrate create add_student_gender
  go run ./cmd/migrate -dsn "postgres://user:pwd@127.0.0.1:5432/school?sslmode=disable" up
`

func main() {
	configFlag := flag.String("config", "configs/server.yaml", "Path to YAML configuration file")
	dsnFlag := flag.String("dsn", "", "PostgreSQL connection string (overrides config file)")
	dirFlag := flag.String("dir", "internal/data/postgres/migrations", "Path to migrations directory")

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

	// Handle 'create' command (does not require DB connection)
	if command == "create" {
		if len(args) < 2 {
			log.Fatalf("错误: 请提供迁移名称，例如: go run ./cmd/migrate create add_user_avatar")
		}
		name := strings.TrimSpace(args[1])
		goose.SetSequential(true)
		if err := goose.Create(nil, *dirFlag, name, "sql"); err != nil {
			log.Fatalf("创建迁移文件失败: %v", err)
		}
		log.Printf("成功在 %s 目录下创建新迁移文件: %s", *dirFlag, name)
		return
	}

	// Load configuration
	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Printf("提示: 读取配置文件 %s 失败 (%v)，将尝试环境变量或 -dsn 参数", *configFlag, err)
		cfg = config.Default()
	}

	dsn := cfg.Database.DSN
	if *dsnFlag != "" {
		dsn = *dsnFlag
	}

	if dsn == "" {
		log.Fatalf("错误: 未配置数据库连接串 (DSN)，请通过 -dsn 参数或 configs/server.yaml 配置")
	}

	ctx := context.Background()
	log.Printf("[MIGRATE] 正在连接数据库: %s ...", postgres.SanitizeDSN(dsn))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("数据库 Ping 失败 (%s): %v", postgres.SanitizeDSN(dsn), err)
	}

	// Execute migration command
	if err := postgres.Migrate(ctx, db, command); err != nil {
		log.Fatalf("执行迁移 [%s] 失败: %v", command, err)
	}

	log.Printf("[MIGRATE] 迁移动作 [%s] 执行完毕。", command)
}
