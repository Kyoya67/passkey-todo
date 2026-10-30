variable "zone_name" {
  type = string
}

variable "record_name" {
  type = string
}

variable "alb_dns_name" {
  type = string
}

variable "alb_zone_id" {
  type = string
}

variable "target_type" {
  type    = string
  default = "alb"

  validation {
    condition     = contains(["alb", "lightsail"], var.target_type)
    error_message = "target_type must be either alb or lightsail."
  }
}

variable "target_ip" {
  type    = string
  default = null
}

variable "tags" {
  type = map(string)
}
