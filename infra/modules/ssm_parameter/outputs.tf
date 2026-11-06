output "parameter_arns" {
  value = merge(
    { for key, parameter in aws_ssm_parameter.config : key => parameter.arn },
    { for key, parameter in aws_ssm_parameter.secure_config : key => parameter.arn }
  )
}
