locals {
  parameter_prefix = "/${var.name}/${var.env}"
}

resource "aws_ssm_parameter" "config" {
  for_each = var.config

  name        = "${local.parameter_prefix}/${each.key}"
  description = "ECS ${each.key} configuration for ${var.name} ${var.env}."
  type        = "String"
  value = each.value
  tier  = "Standard"
  tags  = var.tags
}

resource "aws_ssm_parameter" "secure_config" {
  for_each = toset(["DB_USER", "DB_PASSWORD"])

  name        = "${local.parameter_prefix}/${each.key}"
  description = "ECS ${each.key} secure configuration for ${var.name} ${var.env}."
  type        = "SecureString"
  value_wo         = var.secure_config[each.key]
  value_wo_version = var.secure_config_version
  tier             = "Standard"
  tags             = var.tags
}
