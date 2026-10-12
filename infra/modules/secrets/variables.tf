variable "name" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
  ephemeral = true
}
variable "password_version" { type = number }
variable "tags" { type = map(string) }
