package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/calavera/dkvolume"
)

const (
	redisID       = "_redis"
	socketAddress = "/run/docker/plugins/redis.sock"
)

var (
	defaultDir = filepath.Join(dkvolume.DefaultDockerRootDirectory, redisID)
	dbName     = flag.Int64("db", 0, "Database for writing in")
	password   = flag.String("passwd", "", "Redis DB password")
	root       = flag.String("root", defaultDir, "Docker volumes root directory")
)

func main() {
	var Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		Usage()
		os.Exit(1)
	}

	config := redisConfig{
		ServerURL: flag.Args()[0],
		DbName:    *dbName,
		Password:  *password,
	}

	d := newRedisDriver(*root, config)
	h := dkvolume.NewHandler(d)
	fmt.Printf("Listening on %s\n", socketAddress)
	fmt.Println(h.ServeUnix("root", socketAddress))
}
