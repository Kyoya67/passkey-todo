output "execution_role_arn" { value = aws_iam_role.execution.arn }
output "task_role_arn" { value = aws_iam_role.task.arn }
output "github_actions_role_arn" { value = aws_iam_role.github_actions.arn }
output "portfolio_github_actions_role_arn" { value = aws_iam_role.portfolio_github_actions.arn }
