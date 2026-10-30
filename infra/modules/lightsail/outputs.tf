output "instance_name" {
  value = aws_lightsail_instance.this.name
}

output "static_ip" {
  value = aws_lightsail_static_ip.this.ip_address
}

output "instance_arn" {
  value = aws_lightsail_instance.this.arn
}
