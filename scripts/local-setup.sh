#!/usr/bin/env bash

# Define component versions
ES_VERSION=6.5.3
KIBANA_VERSION=6.5.3
ARC_VERSION=latest
NETWORK=arc

# Remove arc docker network if any
docker network rm ${NETWORK} 1> /dev/null

# Create a docker network
docker network create ${NETWORK} 1> /dev/null

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
docker run -d --rm --name elasticsearch -p 9200:9200 -p 9300:9300 --net=arc -e "discovery.type=single-node" docker.elastic.co/elasticsearch/elasticsearch:${ES_VERSION} 1> /dev/null

# Pull kibana if not found locally
if [[ "$(docker images -q docker.elastic.co/kibana/kibana:${KIBANA_VERSION} 2> /dev/null)" == "" ]]; then
	echo "docker.elastic.co/kibana/kibana:${KIBANA_VERSION} not found, downloading..."
	docker pull "docker.elastic.co/kibana/kibana:${KIBANA_VERSION}" 1> /dev/null
fi

# Start kibana
echo "starting kibana at localhost:5601 ..."
docker run -d --rm --name kibana -p 5601:5601 --net=arc --link elasticsearch docker.elastic.co/kibana/kibana:${KIBANA_VERSION} 1> /dev/null

# Pull arc if not found locally
# if [[ "$(docker images -q appbaseio-confidential/arc:${ARC_VERSION} 2> /dev/null)" == "" ]]; then
# 	echo "appbaseio-confidential/arc:${ARC_VERSION} not found, downloading..."
# 	docker pull "appbaseio-confidential/arc:${ARC_VERSION}" 1> /dev/null
# fi

# Start arc
# echo "starting arc at localhost:8000 ..."
# docker build -t arc:${ARC_VERSION} -f Dockerfile .
# docker run --rm --name arc -p 8000:8000 --env-file .env --net=arc arc:${ARC_VERSION}
