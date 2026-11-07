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

variable "api_target" {
  type        = string
  description = "API DNS target: alb or lightsail."
  default     = "alb"

  validation {
    condition     = contains(["alb", "lightsail"], var.api_target)
    error_message = "api_target must be alb or lightsail."
  }
}

variable "lightsail_static_ip" {
  type        = string
  description = "Lightsail static IPv4 address used when api_target is lightsail."
  default     = null
  nullable    = true
}
