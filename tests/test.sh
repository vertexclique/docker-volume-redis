#!/bin/bash
sudo docker run -d -p 6379:6379 redis
sudo rm -rf /var/lib/docker-volumes/_redis/test
sudo ../docker-volume-redis localhost:6379 & > /dev/null 2>&1
sudo docker run --volume-driver redis --volume test:/data -it alpine sh -c 'echo value1 > /data/key1'
