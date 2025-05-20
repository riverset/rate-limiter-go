APP_NAME=rate-limiter-go
CONFIG_FILE=/Users/prakhar/Desktop/codingChallenges/rate-limiter-go/config.yaml

.PHONY: build start clean

build:
	@echo "Building the Go application..."
	go build -o $(APP_NAME)

start: build
	@echo "Starting the rate limiter application..."
	chmod +x ./start.sh
	./start.sh

clean:
	@echo "Cleaning up..."
	rm -f $(APP_NAME)
