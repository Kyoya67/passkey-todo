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

variable "target" {
  type        = string
  description = "API target: alb or lightsail."

  validation {
    condition     = contains(["alb", "lightsail"], var.target)
    error_message = "target must be alb or lightsail."
  }
}

variable "lightsail_ip" {
  type        = string
  description = "Lightsail static IPv4 address used when target is lightsail."
  default     = null
  nullable    = true
}

variable "tags" {
  type = map(string)
}
