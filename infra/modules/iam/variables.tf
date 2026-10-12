variable "name" { type = string }
variable "secret_arn" { type = string }
variable "log_group_arn" { type = string }
variable "ecr_repository_arn" { type = string }
variable "tags" { type = map(string) }
