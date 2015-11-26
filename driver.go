package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/redis.v3"

	"github.com/calavera/dkvolume"
	"github.com/docker/docker/vendor/src/gopkg.in/fsnotify.v1"
)

type redisConfig struct {
	ServerURL string
	DbName    int64
	Password  string
}

type redisDriver struct {
	root   string
	config redisConfig
	client *redis.Client
	m      *sync.Mutex
}

func newRedisDriver(root string, config redisConfig) redisDriver {
	return redisDriver{
		root:   root,
		config: config,
		client: redis.NewClient(&redis.Options{
			Addr:     config.ServerURL,
			Password: config.Password, // no password set
			DB:       config.DbName,   // use default DB
		}),
		m: &sync.Mutex{},
	}
}

func (d redisDriver) Create(r dkvolume.Request) dkvolume.Response {
	log.Printf("Creating Redis volume %s\n", r.Name)
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	err := os.MkdirAll(m, 0755)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	log.Printf("Checking Redis...\n")
	pong, err := d.client.Ping().Result()
	if err != nil {
		fmt.Println(pong, err)
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{}
}

func (d redisDriver) Remove(r dkvolume.Request) dkvolume.Response {
	log.Printf("Removing Redis volume %s\n", r.Name)
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	err := os.RemoveAll(m)

	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{}
}

func (d redisDriver) Path(r dkvolume.Request) dkvolume.Response {
	return dkvolume.Response{Mountpoint: d.mountpoint(r.Name)}
}

func (d redisDriver) createUpdateFile(filename string) {
	data, errf := ioutil.ReadFile(filename)
	if errf != nil {
		fmt.Println(errf)
	}
	err := d.client.Set(filename, data, 0).Err()
	if err != nil {
		fmt.Println(err)
	}
}

func (d redisDriver) deleteFile(filename string) {
	err := d.client.Del(filename).Err()
	if err != nil {
		fmt.Println(err)
	}
}

func (d redisDriver) walker(m string) {
	fileList := []string{}
	err := filepath.Walk(m, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}

	for _, file := range fileList {
		if _, err := os.Stat(file); err == nil {
			d.createUpdateFile(file)
		}
	}
}

func (d redisDriver) fsnotifier(m string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					log.Println("deleted file:", event.Name)
					d.deleteFile(event.Name)
				} else {
					log.Println("file:", event.Name)
					d.createUpdateFile(event.Name)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(m)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func (d redisDriver) Mount(r dkvolume.Request) dkvolume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	log.Printf("Mounting volume %s on %s\n", r.Name, m)

	err := d.client.Set(r.Name, m, 0).Err()
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	go func() {
		d.walker(m)
	}()

	go func() {
		d.fsnotifier(m)
	}()

	return dkvolume.Response{Mountpoint: m}
}

func (d redisDriver) Unmount(r dkvolume.Request) dkvolume.Response {
	return dkvolume.Response{}
}

func (d *redisDriver) mountpoint(name string) string {
	return filepath.Join(d.root, name)
}
