variable "name" {
  type = string
}

variable "availability_zone" {
  type = string
}

variable "blueprint_id" {
  type = string
}

variable "bundle_id" {
  type = string
}

variable "key_pair_name" {
  type    = string
  default = null
}

variable "user_data" {
  type    = string
  default = null
}

variable "ssh_cidr_blocks" {
  type    = list(string)
  default = []
}

variable "tags" {
  type = map(string)
}
