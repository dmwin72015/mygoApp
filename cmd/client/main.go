package main

import (
	"flag"

	"mygo-app/internal/client"
)

func main() {
	var (
		configFlag = flag.String("config", "", "Path to YAML client configuration file (default: configs/client.yaml if exists)")
		apiFlag    = flag.String("api", "", "Remote backend API base URL (e.g. http://127.0.0.1:8099)")
		driverFlag = flag.String("driver", "", "Data driver mode: 'remote' or 'sqlite'")
		dbFlag     = flag.String("db", "", "SQLite database file path (when driver=sqlite)")
	)
	flag.StringVar(configFlag, "c", "", "Short alias for -config")
	flag.Parse()

	client.RunWithOptions(client.Options{
		ConfigFile: *configFlag,
		APIURL:     *apiFlag,
		Driver:     *driverFlag,
		SQLitePath: *dbFlag,
	})
}
