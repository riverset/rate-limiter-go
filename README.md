# Rate Limiter Go

A Go implementation of various rate limiting algorithms with support for different backends.

## Setup

To set up the project, follow these steps:

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/riverset/rate-limiter-go
    cd rate-limiter-go
    ```

2.  **Install dependencies:**
    Navigate to the project root directory and run:
    ```bash
    go mod tidy
    ```
    This command will download all necessary dependencies defined in the `go.mod` file.

3.  **Configure the application:**
    The application uses a `config.yaml` file for configuration. An example configuration might look like this:
    ```yaml
    # Example config.yaml
    limiters:
      - key: "user_login"
        algorithm: "token_bucket"
        backend: "redis"
        capacity: 100
        rate: 10
        unit: "second"
        redis:
          address: "localhost:6379"
          password: ""
          db: 0
      - key: "api_requests"
        algorithm: "fixed_window_counter"
        backend: "inmemory"
        window: 60
        limit: 1000
    ```
    Update the `config.yaml` file with your desired rate limiter configurations and backend details.

**Configuration Options:**

Each limiter configuration in the `limiters` list supports the following common fields:

*   `key` (string, required): A unique identifier for the rate limiter.
*   `algorithm` (string, required): The rate limiting algorithm to use. Supported values are `token_bucket`, `fixed_window_counter`, and `sliding_window_counter`.
*   `backend` (string, required): The storage backend for the rate limiter state. Supported values are `redis` and `inmemory`.

In addition to the common fields, each algorithm requires specific configuration:

*   **Token Bucket (`token_bucket`):**
    *   `capacity` (integer, required): The maximum number of tokens the bucket can hold.
    *   `rate` (integer, required): The number of tokens to add to the bucket per unit of time.
    *   `unit` (string, required): The time unit for the rate. e.g., "second", "minute", "hour".

*   **Fixed Window Counter (`fixed_window_counter`) & Sliding Window Counter (`sliding_window_counter`):**
    *   `window` (integer, required): The duration of the window in seconds.
    *   `limit` (integer, required): The maximum number of requests allowed within the window.

*   **Redis Backend Configuration:**
    If `backend` is `redis`, the following nested fields are required under the `redis` key:
    *   `address` (string, required): The address of the Redis server (e.g., "localhost:6379").
    *   `password` (string, optional): The password for Redis authentication.
    *   `db` (integer, optional): The Redis database to use.

**Supported Backends:**

The rate limiter supports two backends:

*   **`inmemory`**: The rate limiter state is stored in the application's memory. This is suitable for single-instance deployments or testing. State is not shared between multiple instances.
*   **`redis`**: The rate limiter state is stored in a Redis instance. This backend is suitable for distributed deployments where multiple instances of the rate limiter service need to share the same rate limiting state. Redis handles the persistence and distribution of the state.

**Redis for Distributed Rate Limiting:**

When using the `redis` backend, the rate limiter leverages Redis as a centralized store for rate limiting counters and timestamps. This allows multiple instances of your application (each running the rate limiter) to coordinate and enforce limits globally across all instances. The state (like token counts or window counters) is managed within Redis, ensuring consistency even with a horizontally scaled service.

4.  **Build the project:**
    You can build the project using the provided `Makefile`:
    ```bash
    make build
    ```
    Alternatively, you can use the Go command:
    ```bash
    go build -o rate-limiter-app main.go
    ```
    This will create an executable file named `rate-limiter-app` (or `rate-limiter-app.exe` on Windows).

## Features

*   Support for multiple rate limiting algorithms: Token Bucket, Fixed Window Counter, and Sliding Window Counter.
*   Pluggable storage backends: In-memory for simplicity and Redis for distributed deployments.
*   Flexible configuration via a YAML file.
*   Graceful shutdown of backend clients.
*   Integration points for metrics and middleware.

## Supported Algorithms

This library implements the following rate limiting algorithms:

*   **Token Bucket (`token_bucket`):** Allows a burst of requests up to a certain capacity and then limits the rate at which subsequent requests are allowed.
*   **Fixed Window Counter (`fixed_window_counter`):** Divides time into fixed-size windows and limits the number of requests within each window.
*   **Sliding Window Counter (`sliding_window_counter`):** A more refined version of the fixed window counter that smooths out traffic bursts at the window edges by considering a weighted count from the previous window.

Configuration details for each algorithm can be found in the [Configuration Options](#configuration-options) section.

## Supported Backends

The rate limiter can use the following backends to store its state:

*   **In-memory:** The state is stored directly in the application's memory. This is suitable for single-instance applications or testing environments where state persistence or sharing is not required.
*   **Redis:** The state is stored in a Redis instance. This is the recommended backend for distributed systems where multiple instances of your application need to share rate limiting state to enforce global limits. Details on configuring the Redis backend are available in the [Configuration Options](#configuration-options) section.

## Usage

To use the rate limiter in your Go project, you need to import the module and initialize the limiters using your configuration file.

1.  **Import the module:**

    Make sure your project's `go.mod` file includes a dependency on this rate limiter module. If you are using a local path, you might need a `replace` directive in your `go.mod`.

    ```go
    require learn.ratelimiter v0.1.0 // Or the appropriate version/path
    ```

2.  **Initialize and use the limiters:**

    You can initialize the limiters by providing the path to your `config.yaml` file. The `NewLimitersFromConfigPath` function returns a map of limiters (keyed by their `key` from the config) and an `io.Closer` to gracefully shut down backend clients.

    ```go
    package main

    import (
    	"fmt"
    	"log"
    	"os"

    	"learn.ratelimiter/api"
    )

    func main() {
    	configPath := "./config.yaml" // Path to your configuration file

    	limiters, closer, err := api.NewLimitersFromConfigPath(configPath)
    	if err != nil {
    		log.Fatalf("Failed to initialize rate limiters: %v", err)
    	}
    	defer closer.Close()

    	// Get a specific limiter by its key
    	userLoginLimiter, ok := limiters["user_login"]
    	if !ok {
    		log.Fatalf("Limiter 'user_login' not found")
    	}

    	// Check if a request is allowed
    	allowed, err := userLoginLimiter.Allow("user123") // Use a unique identifier for the client/resource
    	if err != nil {
    		log.Printf("Error checking rate limit: %v", err)
    		// Handle error appropriately
    	}

    	if allowed {
    		fmt.Println("Request allowed for user123")
    		// Process the request
    	} else {
    		fmt.Println("Request denied for user123 - rate limited")
    		// Return a rate limit exceeded response
    	}

    	// You can access other limiters defined in your config similarly
    	// apiRequestsLimiter, ok := limiters["api_requests"]
    	// ...
    }
    ```

Remember to handle errors and close the `io.Closer` when your application exits to ensure proper shutdown of backend clients like Redis.

## Project Structure

The project is organized into the following main directories:

*   `api/`: Contains the main API for initializing and using the rate limiters.
*   `config/`: Holds the configuration loading logic and structures.
*   `internal/`: Contains internal implementations of rate limiting algorithms and backend interactions.
    *   `factory/`: Factories for creating different rate limiter instances.
    *   `fixedcounter/`: Implementation of the fixed window counter algorithm.
    *   `slidingwindowcounter/`: Implementation of the sliding window counter algorithm.
    *   `tokenbucket/`: Implementation of the token bucket algorithm.
    *   `inmemory/`: In-memory backend implementations for algorithms.
    *   `redis/`: Redis backend implementations for algorithms.
*   `metrics/`: Contains code related to metrics and monitoring.
*   `middleware/`: Provides middleware for integrating the rate limiter into web frameworks.
*   `types/`: Defines common types and interfaces used throughout the project.

Key files include:

*   `main.go`: The entry point of the application.
*   `config.yaml`: The default configuration file.
*   `Makefile`: Contains build commands.
*   `README.md`: This file.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
