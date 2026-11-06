variable "name" { type = string }
variable "config" { type = map(string) }
variable "secure_config" {
  type      = map(string)
  sensitive = true
  ephemeral = true
}
variable "secure_config_version" { type = number }
variable "tags" { type = map(string) }
