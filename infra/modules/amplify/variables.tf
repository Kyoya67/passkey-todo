variable "name" {
  type = string
}

variable "branch_name" {
  type = string
}

variable "domain_name" {
  type = string
}

variable "stage" {
  type    = string
  default = "PRODUCTION"
}

variable "tags" {
  type = map(string)
}
