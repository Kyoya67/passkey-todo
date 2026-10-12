output "public_subnet_ids" { value = [for az in var.availability_zones : aws_subnet.public[az].id] }
output "private_subnet_ids" { value = [for az in var.availability_zones : aws_subnet.private[az].id] }
