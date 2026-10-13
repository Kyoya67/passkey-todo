variable "vpc_id" { type = string }
variable "availability_zones" { type = list(string) }
variable "public_cidrs" { type = list(string) }
variable "private_cidrs" { type = list(string) }
variable "tags" { type = map(string) }
