provider "aws" {
  profile = "test-user-profile"
  region  = "eu-west-1"
}

resource "aws_instance" "web" {
  ami           = "ami-0ce8c2b29fcc8a946"
  instance_type = "t2.small"

  tags = {
    Name        = "web-server"
    Environment = "production"
  }

  root_block_device {
    volume_size = 28
    volume_type = "gp3"
  }
}

resource "aws_instance" "db" {
  ami           = "ami-0ce8c2b29fcc8a146"
  instance_type = "t3.large"

  tags = {
    Name        = "db-server"
    Environment = "production"
  }

  root_block_device {
    volume_size = 26
    volume_type = "gp4"
  }
}
