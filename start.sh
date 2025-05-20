#!/bin/bash

REDIS_CONTAINER_NAME="rate-limiter-redis"
CONFIG_FILE="/Users/prakhar/Desktop/codingChallenges/rate-limiter-go/config.yaml"
APP_NAME="rate-limiter-go"

# Default log level
LOG_LEVEL="info"

# Check for a log level argument
if [ "$#" -gt 0 ]; then
    LOG_LEVEL="$1"
fi

echo "Stopping and removing existing Redis container (if any)..."
docker stop $REDIS_CONTAINER_NAME > /dev/null 2>&1
docker rm $REDIS_CONTAINER_NAME > /dev/null 2>&1

echo "Starting Redis container..."
docker run --name $REDIS_CONTAINER_NAME -d -p 6379:6379 redis:latest

echo "Waiting for Redis to start..."
sleep 5 # Give Redis a few seconds to initialize

echo "Building the Go application..."
go build -o $APP_NAME

if [ $? -ne 0 ]; then
    echo "Go build failed. Exiting."
    exit 1
fi

echo "Starting the rate limiter application with log level '$LOG_LEVEL'..."
./$APP_NAME --config $CONFIG_FILE --log-level $LOG_LEVEL

# Optional: Add cleanup for the Redis container when the app stops
# echo "Stopping Redis container..."
# docker stop $REDIS_CONTAINER_NAME
# docker rm $REDIS_CONTAINER_NAME
