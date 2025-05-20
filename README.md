# Rate Limiter Go

A Go implementation of various rate limiting algorithms with support for different backends.

## Features

*   Support for multiple rate limiting algorithms: Token Bucket, Fixed Window Counter, and Sliding Window Counter.
*   Pluggable storage backends: In-memory for simplicity and Redis for distributed deployments.
*   Flexible configuration via a YAML file.
*   Graceful shutdown of backend clients.
*   Integration points for metrics and middleware.

## Supported Algorithms

This library implements the following rate limiting algorithms:

### Token Bucket (`token_bucket`)

The Token Bucket algorithm allows a burst of requests up to a certain capacity and then limits the rate at which subsequent requests are allowed. Tokens are added to the bucket at a fixed rate. A request is allowed only if there are enough tokens in the bucket.

*   **Characteristics:** Good for smoothing out bursts of traffic. Allows for immediate processing of requests as long as tokens are available.
*   **Use Cases:** API rate limiting where occasional bursts are acceptable, controlling the rate of message sending.

### Fixed Window Counter (`fixed_window_counter`)

The Fixed Window Counter algorithm divides time into fixed-size windows (e.g., 60 seconds). It counts the number of requests within each window. If the count exceeds a predefined limit within the current window, further requests are denied until the next window starts.

*   **Characteristics:** Simple to implement and understand. Can lead to traffic bursts at the beginning of each window.
*   **Use Cases:** Simple request rate limiting per user or IP address, preventing denial-of-service attacks.

### Sliding Window Counter (`sliding_window_counter`)

The Sliding Window Counter algorithm is a more refined version of the fixed window counter. It smooths out traffic bursts at the window edges by considering a weighted count from the previous window. It typically uses timestamps of requests within a window.

*   **Characteristics:** Provides a smoother rate limiting enforcement compared to the fixed window counter, reducing the "thundering herd" problem at window boundaries. More complex to implement, especially in distributed systems.
*   **Use Cases:** More accurate and smoother rate limiting for APIs and services, preventing bursts at window edges.

Configuration details for each algorithm can be found in the [Configuration Options](#configuration-options) section.

## Supported Backends

The rate limiter can use the following backends to store its state:

### In-memory (`inmemory`)

The state is stored directly in the application's memory.

*   **Characteristics:** Simple, fast, and suitable for single-instance applications or testing environments.
*   **Use Cases:** Development, testing, and applications where state persistence or sharing across multiple instances is not required.

### Redis (`redis`)

The state is stored in a Redis instance.

*   **Characteristics:** Suitable for distributed deployments where multiple instances of your application need to share the same rate limiting state to enforce global limits. Leverages Redis's data structures and atomic operations (via Lua scripts) for efficient and consistent rate limiting.
*   **Use Cases:** Production deployments of scalable services requiring distributed rate limiting.

### Memcache (`memcache`)

*(Note: Memcache backend is planned but not yet implemented.)*

The state would be stored in a Memcache instance.

*   **Characteristics:** Similar to Redis in providing a distributed cache, but with a simpler data model (key-value).
*   **Use Cases:** Distributed rate limiting in environments where Memcache is the preferred caching solution.

Details on configuring each backend are available in the [Configuration Options](#configuration-options) section.

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
        unit: "second" # Note: The 'unit' field is currently illustrative in config; the code assumes rate is per second for Token Bucket. See development notes.
        redis:
          address: "localhost:6379"
          password: ""
          db: 0
      - key: "api_requests"
        algorithm: "fixed_window_counter"
        backend: "inmemory"
        window: "60s" # e.g., 60 seconds, 1m, 5m
        limit: 1000
      - key: "sliding_api_requests"
        algorithm: "sliding_window_counter"
        backend: "redis"
        window: "1m" # e.g., 60 seconds, 1m, 5m
        limit: 500
        redis:
          address: "localhost:6379"
          password: ""
          db: 0
    ```
    Update the `config.yaml` file with your desired rate limiter configurations and backend details.

**Configuration Options:**

Each limiter configuration in the `limiters` list supports the following common fields:

*   `key` (string, required): A unique identifier for the rate limiter instance. This key is used to retrieve the specific limiter.
*   `algorithm` (string, required): The rate limiting algorithm to use. Supported values are `token_bucket`, `fixed_window_counter`, and `sliding_window_counter`.
*   `backend` (string, required): The storage backend for the rate limiter state. Supported values are `inmemory` and `redis`. (`memcache` is planned).

In addition to the common fields, each algorithm requires specific configuration parameters:

*   **Token Bucket (`token_bucket`):**
    *   `capacity` (integer, required): The maximum number of tokens the bucket can hold.
    *   `rate` (integer, required): The number of tokens to add to the bucket per second. (Note: The `unit` field in the example config is currently illustrative; the code implements rate as tokens per second).

*   **Fixed Window Counter (`fixed_window_counter`) & Sliding Window Counter (`sliding_window_counter`):**
    *   `window` (string, required): The duration of the window (e.g., "60s", "1m", "5m"). Must be a valid Go time.Duration string.
    *   `limit` (integer, required): The maximum number of requests allowed within the window.

Backend-specific configuration is nested under the `redis` or `memcache` keys:

*   **Redis Backend Configuration (`redis`):**
    If `backend` is `redis`, the following nested fields are required under the `redis` key:
    *   `address` (string, required): The address of the Redis server (e.g., "localhost:6379").
    *   `password` (string, optional): The password for Redis authentication.
    *   `db` (integer, optional): The Redis database to use.

*   **Memcache Backend Configuration (`memcache`):**
    If `backend` is `memcache`, the following nested fields are required under the `memcache` key:
    *   `addresses` (list of strings, required): A list of Memcache server addresses (e.g., `["localhost:11211"]`).

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
    	"context"
    	"fmt"
    	"log"
    	"os"
    	"time"

    	"learn.ratelimiter/api"
    	"learn.ratelimiter/types" // Import types to use types.Limiter
    )

    func main() {
    	configPath := "./config.yaml" // Path to your configuration file

    	limiters, closer, err := api.NewLimitersFromConfigPath(configPath)
    	if err != nil {
    		log.Fatalf("Failed to initialize rate limiters: %v", err)
    	}
    	// Defer closing the backend clients when main exits
    	defer func() {
    		if cerr := closer.Close(); cerr != nil {
    			log.Printf("Error closing backend clients: %v", cerr)
    		}
    	}()


    	// Get a specific limiter by its key
    	userLoginLimiter, ok := limiters["user_login"]
    	if !ok {
    		log.Fatalf("Limiter 'user_login' not found")
    	}

    	// Example usage in a loop
    	identifier := "user123"
    	fmt.Printf("Testing rate limiter '%s' for identifier '%s'\n", "user_login", identifier)

    	for i := 0; i < 15; i++ {
    		// Use a context with a timeout or cancellation
    		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    		allowed, err := userLoginLimiter.Allow(ctx, identifier)
    		cancel() // Always call cancel to release resources

    		if err != nil {
    			log.Printf("Error checking rate limit for request %d: %v", i+1, err)
    			// Handle error appropriately
    			continue
    		}

    		if allowed {
    			fmt.Printf("Request %d allowed\n", i+1)
    			// Process the request
    		} else {
    			fmt.Printf("Request %d denied - rate limited\n", i+1)
    			// Return a rate limit exceeded response
    		}

    		// Simulate some delay between requests if needed
    		// time.Sleep(100 * time.Millisecond)
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
    *   `memcache/`: *(Planned)* Memcache backend implementations.
*   `metrics/`: Contains code related to metrics and monitoring.
*   `middleware/`: Provides middleware for integrating the rate limiter into web frameworks.
*   `types/`: Defines common types and interfaces used throughout the project.

Key files include:

*   `main.go`: The entry point of the example HTTP application.
*   `config.yaml`: The default configuration file example.
*   `Makefile`: Contains build commands.
*   `README.md`: This file.
*   `LICENSE`: The project's license file.

## How to Contribute

We welcome contributions to improve this rate limiting library! Here are some ways you can contribute:

*   **Implement New Algorithms:** Add support for other rate limiting algorithms (e.g., Leaky Bucket).
*   **Add New Backends:** Implement support for additional storage backends (e.g., Memcache, PostgreSQL, etcd).
*   **Improve Existing Implementations:** Optimize existing algorithms or backend interactions for performance and efficiency.
*   **Enhance Documentation:** Improve the README, add examples, or write GoDoc comments.
*   **Add More Tests:** Increase test coverage, add benchmark tests, or set up integration tests for backends.
*   **Implement Production Readiness Features:** Add features like detailed metrics (Prometheus integration), tracing, improved error handling, or backend health checks.

To contribute:

1.  Fork the repository.
2.  Create a new branch for your feature or bug fix.
3.  Make your changes, following the project's coding style and conventions.
4.  Write tests for your changes.
5.  Ensure all tests pass (`go test ./...`).
6.  Submit a pull request with a clear description of your changes.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
