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
	"mygo-app/internal/data/crypto"
	"mygo-app/internal/data/mysql"
	"mygo-app/internal/data/postgres"
	"mygo-app/internal/domain"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const usage = `启明培训 · 数据库迁移与运维管理工具 (Database Migration & Ops CLI)

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
  seed                 执行演示种子数据脚本 (仅限测试/开发环境使用)
  create-admin         创建或重置管理员账号 (适用于生产环境初始配置或密码重置)

常用参数 (Flags):
  -config string       配置文件路径 (默认 "configs/server.yaml")
  -driver string       数据库驱动: "postgres" 或 "mysql" (默认从配置文件读取)
  -dsn string          数据库连接串 (覆写配置文件中的 DSN)
  -dir string          迁移脚本所在目录 (默认根据驱动自动选择)
  -admin-user string   管理员用户名 (默认 "admin"，仅用于 create-admin)
  -admin-pass string   管理员密码 (留空则在终端交互提示，仅用于 create-admin)
  -admin-name string   管理员姓名 (默认 "系统管理员"，仅用于 create-admin)

示例 (Examples):
  # 1. 执行数据库结构升级
  go run ./cmd/migrate up

  # 2. 生产环境创建/重置管理员
  go run ./cmd/migrate create-admin -admin-user admin -admin-pass "P@ssw0rd2026!"

  # 3. 开发环境注入演示种子数据
  go run ./cmd/migrate seed
`

func main() {
	configFlag := flag.String("config", "configs/server.yaml", "Path to YAML configuration file")
	driverFlag := flag.String("driver", "", "Database driver: 'postgres' or 'mysql' (overrides config file)")
	dsnFlag := flag.String("dsn", "", "Database connection string (overrides config file)")
	dirFlag := flag.String("dir", "", "Path to migrations directory")

	adminUserFlag := flag.String("admin-user", "admin", "Admin username for create-admin")
	adminPassFlag := flag.String("admin-pass", "", "Admin password for create-admin (interactive prompt if empty)")
	adminNameFlag := flag.String("admin-name", "系统管理员", "Admin display name for create-admin")

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
		log.Printf("[CLI] 正在连接 PostgreSQL 数据库: %s ...", maskedDSN)
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("连接 PostgreSQL 失败: %v", err)
		}
		defer db.Close()

		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("数据库 Ping 失败 (%s): %v", maskedDSN, err)
		}

		handleDBCommand(ctx, db, driver, command, *adminUserFlag, *adminPassFlag, *adminNameFlag)

	case "mysql":
		maskedDSN = mysql.SanitizeDSN(dsn)
		normDSN := mysql.NormalizeDSN(dsn)
		log.Printf("[CLI] 正在连接 MySQL 数据库: %s ...", maskedDSN)
		db, err = sql.Open("mysql", normDSN)
		if err != nil {
			log.Fatalf("连接 MySQL 失败: %v", err)
		}
		defer db.Close()

		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("数据库 Ping 失败 (%s): %v", maskedDSN, err)
		}

		handleDBCommand(ctx, db, driver, command, *adminUserFlag, *adminPassFlag, *adminNameFlag)

	default:
		log.Fatalf("不支持的数据库驱动: %s (支持: postgres, mysql)", driver)
	}
}

func handleDBCommand(ctx context.Context, db *sql.DB, driver, command, adminUser, adminPass, adminName string) {
	switch command {
	case "seed":
		log.Printf("[CLI] 正在注入演示种子数据 (Demo Seed Data) ...")
		var err error
		if driver == "mysql" {
			err = mysql.Seed(ctx, db)
		} else {
			err = postgres.Seed(ctx, db)
		}
		if err != nil {
			log.Fatalf("注入演示数据失败: %v", err)
		}
		log.Printf("[CLI] ✅ 成功注入演示种子数据！")

	case "create-admin":
		username := strings.TrimSpace(adminUser)
		if username == "" {
			username = "admin"
		}
		password := strings.TrimSpace(adminPass)
		if password == "" {
			fmt.Printf("请输入管理员 [%s] 的密码 (直接回车默认使用 admin123): ", username)
			var input string
			_, _ = fmt.Scanln(&input)
			input = strings.TrimSpace(input)
			if input == "" {
				password = "admin123"
			} else {
				password = input
			}
		}

		if len([]rune(password)) < 6 {
			log.Fatalf("错误: 密码长度至少为 6 位")
		}

		salt, hash, err := crypto.HashPassword(password)
		if err != nil {
			log.Fatalf("生成密码哈希失败: %v", err)
		}

		code := domain.NewID("usr")
		displayName := strings.TrimSpace(adminName)
		if displayName == "" {
			displayName = "系统管理员"
		}

		if driver == "mysql" {
			query := `INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
			          VALUES (?, ?, ?, 'admin', ?, ?, NOW())
			          ON DUPLICATE KEY UPDATE
			          display_name = VALUES(display_name), role = 'admin', salt = VALUES(salt), hash = VALUES(hash)`
			_, err = db.ExecContext(ctx, query, code, username, displayName, salt, hash)
		} else {
			query := `INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
			          VALUES ($1, $2, $3, 'admin', $4, $5, CURRENT_TIMESTAMP)
			          ON CONFLICT (username) DO UPDATE
			          SET display_name = EXCLUDED.display_name, role = 'admin', salt = EXCLUDED.salt, hash = EXCLUDED.hash`
			_, err = db.ExecContext(ctx, query, code, username, displayName, salt, hash)
		}

		if err != nil {
			log.Fatalf("创建/重置管理员账号失败: %v", err)
		}
		log.Printf("[CLI] ✅ 成功设置管理员账号: 用户名=%s, 显示名称=%s, 角色=admin", username, displayName)

	default:
		// Standard goose migration actions
		var err error
		if driver == "mysql" {
			err = mysql.Migrate(ctx, db, command)
		} else {
			err = postgres.Migrate(ctx, db, command)
		}
		if err != nil {
			log.Fatalf("执行迁移 [%s] 失败: %v", command, err)
		}
		log.Printf("[CLI] 迁移动作 [%s] 执行完毕。", command)
	}
}
