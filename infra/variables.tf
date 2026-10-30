locals {
  account_id = "588081156679"
  region     = "ap-northeast-1"

  project = "kyoya67-todo"

  common_tags = {
    Project   = local.project
    ManagedBy = "Terraform"
  }
}

variable "db_username" {
  type        = string
  description = "Master username for RDS PostgreSQL."
  default     = "todo_admin"
}

variable "db_password" {
  type        = string
  sensitive   = true
  description = "Master password for RDS PostgreSQL. Supply through TF_VAR_db_password."
  ephemeral   = true
  nullable    = false
}

variable "db_password_version" {
  type        = number
  description = "Increment this value when rotating the RDS master password."
  default     = 1
}

variable "backend_image" {
  type        = string
  description = "Backend container image URI in ECR."
  default     = ""
}

variable "migration_image" {
  type        = string
  description = "Migration container image URI in ECR."
  default     = ""
}

variable "backend_port" {
  type        = number
  description = "Port exposed by the backend container."
  default     = 8080
}

variable "enable_lightsail" {
  type        = bool
  description = "Create the staged Lightsail host for the containerized backend."
  default     = false
}

variable "lightsail_availability_zone" {
  type        = string
  description = "Availability zone for the Lightsail instance."
  default     = "ap-northeast-1a"
}

variable "lightsail_blueprint_id" {
  type        = string
  description = "Lightsail blueprint used to create the host OS."
  default     = "ubuntu_24_04"
}

variable "lightsail_bundle_id" {
  type        = string
  description = "Lightsail bundle used for the initial 1 GB host."
  default     = "micro_3_0"
}

variable "lightsail_key_pair_name" {
  type        = string
  description = "Existing Lightsail key pair used for SSH access."
  default     = null
}

variable "lightsail_ssh_cidr_blocks" {
  type        = list(string)
  description = "CIDR blocks allowed to connect to SSH. Keep empty until an access range is selected."
  default     = []
}

variable "api_dns_target" {
  type        = string
  description = "Target for the API Route 53 record. Keep ALB until Lightsail is validated."
  default     = "alb"

  validation {
    condition     = contains(["alb", "lightsail"], var.api_dns_target)
    error_message = "api_dns_target must be either alb or lightsail."
  }
}
