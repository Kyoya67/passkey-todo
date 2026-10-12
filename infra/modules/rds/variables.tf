variable "identifier" { type = string }
variable "subnet_ids" { type = list(string) }
variable "security_group_id" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
  ephemeral = true
}
variable "password_version" { type = number }
variable "tags" { type = map(string) }
