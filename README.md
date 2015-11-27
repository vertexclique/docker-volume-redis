# Docker volume plugin for Redis

This plugin uses Redis as file-only storage for containers.

## Installation

Use below;

```
$ go get github.com/vertexclique/docker-volume-redis
```

## Usage

You can simply:

1 - Start the plugin using this command:

```
$ sudo docker-volume-redis localhost:6379
```

2 - Start your docker containers with the option `--volume-driver=redis` and use the first part of `--volume` to specify the remote volume that you want to connect to:

```
$ sudo docker run --volume-driver redis --volume testing:/data alpine ash
```

You can manipulate your files in containers and see how the Redis KV store is changing.

Current Redis value limitation ends with 512mb per KV. So probably files which is more than 512 mb won't be inserted correctly.

## Running Tests

```
./runtests.sh
```

## TODO

1. Skip directories when KV insert (I forgot it)
2. rename event from inotify needs deletion too.

## LICENSE

MIT
