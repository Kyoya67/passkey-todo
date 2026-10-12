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

variable "backend_port" {
  type        = number
  description = "Port exposed by the backend container."
  default     = 8080
}
