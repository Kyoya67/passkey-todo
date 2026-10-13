output "app_id" {
  value = aws_amplify_app.this.id
}

output "default_domain" {
  value = aws_amplify_app.this.default_domain
}

output "branch_url" {
  value = "https://${var.branch_name}.${aws_amplify_app.this.default_domain}"
}

output "custom_domain" {
  value = var.domain_name
}
