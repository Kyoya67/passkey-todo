variable "name" { type = string }
variable "alb_arn" { type = string }
variable "tags" { type = map(string) }

resource "aws_wafv2_web_acl" "this" {
  name  = "${var.name}-auth-rate-limit"
  scope = "REGIONAL"

  default_action { allow {} }

  rule {
    name     = "authentication-rate-limit"
    priority = 0

    action { block {} }

    statement {
      rate_based_statement {
        limit              = 100
        aggregate_key_type = "IP"
        evaluation_window_sec = 60

        scope_down_statement {
          byte_match_statement {
            search_string = "/auth/"
            positional_constraint = "STARTS_WITH"
            field_to_match { uri_path {} }
            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "AuthenticationRateLimit"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "TodoAPIWebACL"
    sampled_requests_enabled   = true
  }

  tags = var.tags
}

resource "aws_wafv2_web_acl_association" "this" {
  resource_arn = var.alb_arn
  web_acl_arn  = aws_wafv2_web_acl.this.arn
}
