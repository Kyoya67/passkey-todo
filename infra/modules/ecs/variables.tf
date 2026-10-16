variable "name" { type = string }
variable "env" {
  type    = string
  default = "stg"
}
variable "image" { type = string }
variable "migration_image" { type = string }
variable "port" { type = number }
variable "cluster_name" { type = string }
variable "subnet_ids" { type = list(string) }
variable "security_group_id" { type = string }
variable "execution_role_arn" { type = string }
variable "task_role_arn" { type = string }
variable "secret_arn" { type = string }
variable "log_group_name" { type = string }
variable "database_host" { type = string }
variable "redis_host" { type = string }
variable "cors_allowed_origins" { type = string }
variable "csrf_allowed_origins" { type = string }
variable "region" { type = string }
variable "tags" { type = map(string) }
