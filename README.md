# EC2 Drift Detector

This tool detects drift between Terraform state and configuration, specifically for EC2 instances. It helps identify mismatches between your deployed infrastructure and the desired state as defined in your Terraform files.

## Getting Started

### Clone the Repository
- `git clone https://github.com/oldmonad/ec2Drift`

- `cd ec2drift`

- go version `1.19+`

### Install Dependencies
- `go mod tidy`


## Running the Application
- First, build the binary: `go build -o ec2drift ./cmd/main.go`, this will create an executable file named `ec2drift` in the current directory.

- The application supports both CLI and HTTP interfaces.

- Run the application via the CLI: `./ec2drift run`

- Start the application with http server: `./ec2drift serve --port 8080`

- Run CLI application with comma separated attributes: `./ec2drift run --attributes,security_groups`

- Sample http request: `curl -X POST http://localhost:8080/drift -H "Content-Type: application/json" -d '{}'`

- Supported attributes for drift checks: `ami`, `instance_type`, `security_groups`,`root_block_device.volume_size`,
`root_block_device.volume_type`

- Create a .env file and setup environment variables, check .env.example for reference

## Running Tests
- Unit tests for the core logic be run as follows:
  - For specific modules, use `go test ./internal/app`
  - For specific tests use `go test ./internal/driftchecker -run TestDetectBasicDrift`, `TestDetectBasicDrift` is a test function that detects basic drift between Terraform state and configuration.

- To view and generate coverage report:
  - `go test ./internal/app -cover -v -coverprofile=coverage.out`

  - `go tool cover -html=coverage.out -o coverage.html`

  - `open coverage.html`

### Approach decisions and challenges faced

The **Drift Checker** addresses an infrastructure management tool for detecting configuration drift between a desired cloud infrastructure state and what actually exists in the cloud environment. The motivation behind this tool was to create a reliable, scalable system that could operate in both command-line and HTTP server modes, giving users flexibility in how they integrate drift detection into their workflows.

At the heart of the tool lies a comparison engine that works by analyzing two datasets: one representing the "desired" infrastructure state, typically extracted from a Terraform state file, and another representing the "current" live state of cloud instances fetched through a provider API such as AWS or GCP. The comparison process is optimized by first transforming both datasets into maps, using the "Name" tag of each instance as the key. This approach allows for fast and deterministic lookups. To ensure consistent behavior and avoid ambiguity, instances that lack a "Name" tag are deliberately skipped.

Once the desired and current states are indexed, the drift checker begins its core operation. For each instance in the desired state, the tool checks whether it still exists in the current environment. If not, it marks the instance as removed. If it does exist, a deeper comparison is launched to examine a specific set of attributes. These comparisons are performed concurrently using goroutines, allowing the application to take advantage of multicore processors and improve throughput, especially when handling large infrastructures.

Each attribute comparison is tailored to its complexity. Simple scalar attributes like `ami` and `instance_type` are directly compared. More complex structures, such as lists of security groups, instance tags, or root block device definitions, require normalization steps like sorting or nested field analysis to determine whether drift has occurred. The results from all comparisons are sent to a buffered channel, which helps avoid contention when many goroutines report findings simultaneously.

Parallelism plays a key role in the design of this system, but it also introduces a degree of unpredictability. Because goroutines complete at different times, the order of the final drift reports is nondeterministic. While this does not affect the accuracy of the results, it does make testing and diffing outputs more difficult. That trade-off was accepted in favor of performance, though it may be revisited in future versions by introducing optional sorting.

The application is designed with two execution modes in mind: a CLI mode and an HTTP server mode. In CLI mode, the user can invoke the drift checker directly via the terminal, specifying options like which attributes to check. In HTTP mode, the tool starts a web server that listens on a configurable port and exposes an endpoint for on-demand drift checks. This architecture makes it easy to plug the drift checker into automation pipelines or integrate it into broader systems.

The main application begins by loading environment configuration using a dedicated library and sets up its logging and validation components. Based on the selected execution mode, it either runs the drift detection process directly or starts the HTTP server. In either case, the core workflow remains consistent: retrieve the current state from the cloud provider, read the desired state from disk, parse both, and execute the drift detection logic. Results are printed in the CLI.

Building this system came with its share of challenges. Managing large numbers of goroutines required care to avoid excessive memory usage or system strain, particularly when working with infrastructure composed of hundreds or thousands of instances. Another challenge was ensuring consistency and correctness in attribute comparison logic, especially for nested or unordered data like tags and security group IDs. To address this, I have implemented normalization techniques such as sorting and explicit key comparison.

Error handling and context management were also important. The tool is designed to respect context cancellation, allowing it to shut down gracefully if a timeout or termination signal is received. This is particularly relevant in automated environments where long-running operations must be interruptible.


### Drift check algorithm analysis

The drift checker package compares two sets of cloud instances: one from an "old" state and another from a "current" state. It matches instances by their "Name" tag. First, the code goes through each list and builds maps using the "Name" tag as the key. This step then converts the lists into maps, allowing fast lookups. Any instance without a "Name" tag is skipped because the tag is used as the unique identifier.

After the maps are built, the function processes the old state by looping over its entries. If an instance from the old state is not found in the current state, the function starts a goroutine to report that the instance was removed. If an instance exists in both states, another goroutine is launched to compare the instance's attributes, as specified in an input list. The code loops over each attribute string and uses a switch statement to handle different cases. For simple attributes like "ami" and "instance\_type," it performs a basic comparison. For more complex attributes such as "security\_groups," tags, and parts of the "root\_block\_device," it uses detailed logic. For example, comparing "security\_groups" requires sorting the slices and then comparing them.

The code also handles instances that appear only in the current state by launching goroutines to mark them as newly added. A WaitGroup makes sure that all these goroutines finish their tasks. A buffered channel collects all the drift reports, and its size is set to avoid blocking during heavy parallel writes. Additionally, the code checks for context cancellation in each goroutine so that it can stop processing if needed.

For each instance, the function loops over the needed attributes. Simple scalar checks run in constant time. However, attributes like "security\_groups" require sorting, which takes longer (approximately O(m log m) for a list of length m). Tag comparisons vary based on whether a specific tag or all tags are checked. Nested objects like "root\_block\_device" are compared by checking each part separately.

Once all goroutines have finished, the channel is closed and the drift reports are gathered into a final list. Because the reports come from different goroutines that finish at different times, their final order is random. This nondeterministic order is acceptable for the task at hand.

Overall, building the maps takes linear time relative to the number of instances, and the subsequent comparisons work in roughly O(n × k) time, where n is the number of instances and k is the number of attributes to check. Some operations might take extra time if sorting or additional iterations are needed. The design uses goroutines to perform these tasks in parallel, which can reduce total processing time on multicore systems. However, creating many goroutines can add some overhead, especially when dealing with a very large number of instances.

TLDR, the drift checker efficiently uses maps for quick lookups and goroutines for concurrent processing. Although the order of results is random and there is some overhead from parallelism, these trade-offs are reasonable for quickly finding discrepancies in a scalable way.

### Codebase implementation details

The application starts in the main package. In the main function, environment variables are loaded using a dedicated library. The application sets up its logging system and retrieves configuration settings from the environment. With these settings, an application instance is created. After that, the main function creates a validator and an HTTP server instance. A command factory is then instantiated, receiving the application instance, validator, HTTP server, and configuration data as dependencies. The factory uses a command library to build a root command, which is extended by subcommands. Finally, the root command is executed, determining whether the application should run its drift detection or serve an HTTP endpoint.

When the command factory builds the CLI commands, it embeds the application instance along with the validator and HTTP server into a Command structure. The factory creates a command that triggers the drift detection process. When this command is run, the validator checks the input parameters, including the desired output format and the attributes that should be verified. Once validated, the application’s Run method is called. Alternatively, another command is set up to start the HTTP server. This command uses the HTTP server instance to start a server that listens on a specified port.

Inside the application layer, the Run method orchestrates the entire drift detection workflow. It first obtains a snapshot of the live cloud state by fetching instances from a cloud provider. This decision is based on a configuration value that selects between different cloud provider implementations, such as AWS or GCP. After acquiring the current state, the application reads a state file from disk to load the desired state. The file contents are then parsed using a parser that understands different input formats. With both the desired state and live state available, the application invokes a drift detection routine. This routine compares both states according to a set of attributes and produces drift reports if discrepancies are found. When drift is detected, these reports are logged and printed, and a specific error is returned or the application may exit if it is running in CLI mode.

The HTTP server implementation provides an alternative way to interact with the drift detection functionality. The server is built by creating an instance that holds a drift handler. The drift handler is responsible for processing HTTP requests at a particular endpoint. When the server starts, it defines an HTTP multiplexer that maps a drift-specific route to the handler. The server is then started with an address created using the specified port. A background routine listens for incoming HTTP connections while also waiting for system signals that indicate the server should shut down. Upon receiving such a signal, the server initiates a graceful shutdown, ensuring that resources are correctly released.

The overall design offers flexibility. One can build the executable from the main package, producing a binary that supports both the CLI and HTTP interfaces. In CLI mode, the application runs a drift check and outputs drift reports based on command-line arguments. In HTTP mode, the application listens for web requests, allowing for remote triggering of the drift check. This implementation combines configuration management, command-line processing, concurrent drift detection, and HTTP server handling into one cohesive system.
