# EC2 Drift Detector

This tool detects drift between Terraform state and configuration, specifically for EC2 instances. It helps identify mismatches between your deployed infrastructure and the desired state as defined in your Terraform files.

## Getting Started

### Clone the Repository
- `git clone https://github.com/oldmonad/ec2Drift.git`
- `cd ec2drift`
- go version `1.19+`

### Install Dependencies
- `go mod tidy`


## Running the Application
- First, build the binary: `go build -o ec2drift ./cmd/main.go`, this will create an executable file named `ec2drift` in the current directory.
- Run the application: `./ec2drift --old-state samples/terraform.tfstate --new-state samples/dev.tfvars`
- Run the application with attributes: `./ec2drift --old-state samples/terraform.tfstate --new-state samples/dev.tfvars --attributes security_groups`
- Supported attributes for drift checks: `ami`, `instance_type`, `security_groups`, `source_dest_check`, `subnet_id`, `vpc_security_group_ids`

## Running Tests
- Unit tests for the core logic be run as follows:
  - For the whole core logic folder, use `go test ./internal/*`
  - For specific modules, use `go test ./internal/comparator`
  - For specific tests use `go test ./internal/comparator -run TestDetectDriftBasicDrift`, `TestDetectDriftBasicDrift` is a test function that detects basic drift between Terraform state and configuration.

- To view and generate coverage report:
  - `go test ./internal/* -cover -v -coverprofile=coverage.out`
  - `go tool cover -html=coverage.out -o coverage.html`
  - `open coverage.html`

### Approach, Decisions, and Challenges faced

This project was all about finding differences between what’s actually running and what’s defined in the Terraform files. Since the tfstate file is plain JSON, I simply converted it to extract the live settings for EC2 instances, which was pretty straightforward.

The Terraform configuration was trickier. To keep things neat, I combined all the settings into one .tfvars file instead of having a bunch of different files. At first, the configuration only included basic details like the application name, AMI, instance type, and how many instances there were. Later on, I added more options like security groups, source/destination checks, subnet IDs, and VPC security group IDs. This gradual change meant I had to make sure the new details didn’t mess up the old data.

One key decision was figuring out how to connect the desired configuration with what was recorded in the state file. Since the frontend Terraform configuration doesn’t give a clear unique ID for each EC2 instance, I chose to use the application name as the key mapped to `index_key` in the tfstate file. This allowed the tool to match each configuration to the right instance and see where things were different.

I also built a flexible command-line interface. The tool lets you input both the state file and the new configuration file, and you can even check only specific details like security groups or instance types. This makes it easy to see changes in just the areas you care about.

Even though these methods worked well, there were some challenges. Matching the fixed configuration to the dynamic state file was hard without a unique ID, and while using the application name worked for now, it might be a problem if the names aren’t unique. Additionally, the tool currently assumes there’s only one EC2 instance per entry because the `no_of_instances` is always set to one. I didn’t have time to handle cases where there might be more than one, so that’s something to improve in the future.
