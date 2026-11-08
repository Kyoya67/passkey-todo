locals {
  account_id = "588081156679"
  region     = "ap-northeast-1"

  project = "kyoya67-todo"

  deployment_target   = "lightsail"
  lightsail_static_ip = "35.74.94.30"
  backend_image       = ""
  migration_image     = ""
  backend_port        = 8080
  is_ecs              = local.deployment_target == "ecs"

  common_tags = {
    Project   = local.project
    ManagedBy = "Terraform"
  }
}

variable "db_username" {
  type        = string
  sensitive   = true
  description = "Master username for RDS PostgreSQL."
  nullable    = false
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
