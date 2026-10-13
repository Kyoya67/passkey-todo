resource "aws_secretsmanager_secret" "database" {
  name                    = var.name
  recovery_window_in_days = 7
  tags                    = var.tags
}

resource "aws_secretsmanager_secret_version" "database" {
  secret_id = aws_secretsmanager_secret.database.id
  secret_string_wo = jsonencode({
    username = var.username
    password = var.password
  })
  secret_string_wo_version = var.password_version
}
