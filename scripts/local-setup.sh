#!/usr/bin/env bash

# Define component versions
ES_VERSION=6.5.3
KIBANA_VERSION=6.5.3
ARC_VERSION=latest

# Stop arc if already running
arc=$(docker ps -a | grep arc:latest | awk '{print $1}')
if [[ $arc != "" ]]; then
	printf "($arc): arc is currently running, stopping ...\n"
	docker stop $arc 1> /dev/null
fi

# Stop kibana if already running
kibana=$(docker ps -a | grep kibana | awk '{print $1}')
if [[ $kibana != "" ]]; then
	printf "($kibana): kibana is currently running, stopping ...\n"
	docker stop $kibana 1> /dev/null
fi

# Stop elasticsearch if already running
elasticsearch=$(docker ps -a | grep elasticsearch | awk '{print $1}')
if [[ $elasticsearch != "" ]]; then
	printf "($elasticsearch): elasticsearch is currently running, stopping ...\n"
	docker stop $elasticsearch 1> /dev/null
fi

# Pull elasticsearch if not found locally
if [[ "$(docker images -q docker.elastic.co/elasticsearch/elasticsearch:${ES_VERSION} 2> /dev/null)" == "" ]]; then
	echo "docker.elastic.co/elasticsearch/elasticsearch:${ES_VERSION} not found, downloading..."
	docker pull "docker.elastic.co/elasticsearch/elasticsearch:${ES_VERSION}" 1> /dev/null
fi

# Start elasticsearch
echo "starting elasticsearch at localhost:9200 ..."
docker run -d --rm --name elasticsearch -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" docker.elastic.co/elasticsearch/elasticsearch:${ES_VERSION} 1> /dev/null

# Pull kibana if not found locally
if [[ "$(docker images -q docker.elastic.co/kibana/kibana:${KIBANA_VERSION} 2> /dev/null)" == "" ]]; then
	echo "docker.elastic.co/kibana/kibana:${KIBANA_VERSION} not found, downloading..."
	docker pull "docker.elastic.co/kibana/kibana:${KIBANA_VERSION}" 1> /dev/null
fi

# Start kibana
echo "starting kibana at localhost:5601 ..."
docker run -d --rm --name kibana -p 5601:5601 --link elasticsearch docker.elastic.co/kibana/kibana:${KIBANA_VERSION} 1> /dev/null

# Start arc
# echo "starting arc at localhost:8000 ..."
# docker run --rm --name arc --env-file .env -p 8000:8000 arc:latest
