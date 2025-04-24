configurations = [
  {
    application_name  = "example_app_server-dev-one"
    ami               = "ami-0df368112825f8d8f"
    instance_type     = "t2.small"
    no_of_instances   = 1
    source_dest_check = false
    security_groups   = ["sg-0123456789abcdef", "sg-abcdef01234567890"]
    subnet_id         = "subnet-0123456789abcdef"
  },
  {
    application_name = "example_web_server-dev-two"
    ami              = "ami-0df368112825f8d8f"
    instance_type    = "t2.medium"
    no_of_instances  = 1
    security_groups  = ["sg-abcdef01234567890"]
    vpc_security_group_ids = [
      "sg-0da280359c67fd045", "sg-0da280359c67fd046"
    ]
  }
]
