package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	conns  *[]string
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
		conns: &[]string{},
		m:     &sync.Mutex{},
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

	*d.conns = append(*d.conns, m)

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

func (d redisDriver) createToRedis(filename string, realpath string) {
	data, errf := ioutil.ReadFile(realpath)
	if errf != nil {
		fmt.Println(errf)
	}

	strdata := strings.TrimSpace(string(data))

	redisData, err := d.client.Get(filename).Result()
	if err != nil {
		redisData = ""
	}

	if len([]byte(redisData)) != len(data) {
		err := d.client.Set(filename, strdata, 0).Err()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (d redisDriver) deleteFromRedis(filename string) {
	log.Printf("deletion")
	err := d.client.Del(filename).Err()
	if err != nil {
		fmt.Println(err)
	}
}

func (d redisDriver) walker(m string, realname string) {
	fileList := []string{}
	err := filepath.Walk(m, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}

	for _, file := range fileList {
		if finfo, err := os.Stat(file); err == nil {
			if !finfo.IsDir() {
				filename := strings.Replace(file, m+"/", "", -1)
				d.createToRedis(filename, file)
			}
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
				// log.Println("event:", event)
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					//log.Println("deleted file:", event.Name)
					filename := strings.Replace(event.Name, m+"/", "", -1)
					d.deleteFromRedis(filename)
					for _, conn := range *d.conns {
						os.RemoveAll(conn + string(os.PathSeparator) + filename)
					}
				} else {
					log.Println("file:", event.Name)

					filename := strings.Replace(event.Name, m+"/", "", -1)
					d.createToRedis(filename, event.Name)
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

func pos(slice []string, secondValue string) int {
	for p, v := range slice {
		if v == secondValue {
			return p
		}
	}
	return -1
}

func getKeys(d redisDriver) []string {
	keys, err := d.client.Keys("*").Result()
	if err != nil {
		log.Fatal(err)
	}
	return keys
}

func (d redisDriver) syncRedis() {
	keys := getKeys(d)
	for _, conn := range *d.conns {
		for _, key := range keys {

			filename := conn + string(os.PathSeparator) + key

			if strings.Contains(key, string(os.PathSeparator)) {
				err := os.MkdirAll(filepath.Dir(filename), os.FileMode(777))
				if err != nil {
					fmt.Println(err)
				}
			}

			data, err := d.client.Get(key).Result()
			if err != nil {
				fmt.Println(err)
			}

			if _, err := os.Stat(filename); err == nil {

			} else {
				// file doesnt exist
				ioutil.WriteFile(filename, []byte(data), os.FileMode(777))
			}
		}
	}
}

func (d redisDriver) watchRedis() {
	ticker := time.NewTicker(1 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				d.syncRedis()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (d redisDriver) Mount(r dkvolume.Request) dkvolume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	log.Printf("Mounting volume %s on %s\n", r.Name, m)

	log.Printf("conns: %s", *d.conns)

	go func() {
		d.walker(m, r.Name)
	}()

	go func() {
		d.syncRedis()
	}()

	d.watchRedis()

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
