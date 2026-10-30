resource "aws_route53_zone" "this" {
  name = var.zone_name

  tags = var.tags
}

resource "aws_route53_record" "alb" {
  zone_id = aws_route53_zone.this.zone_id
  name    = var.record_name
  type    = "A"

  dynamic "alias" {
    for_each = var.target_type == "alb" ? [1] : []

    content {
      name                   = var.alb_dns_name
      zone_id                = var.alb_zone_id
      evaluate_target_health = true
    }
  }

  records = var.target_type == "lightsail" ? [var.target_ip] : null
  ttl     = var.target_type == "lightsail" ? 60 : null

  lifecycle {
    precondition {
      condition     = var.target_type != "lightsail" || var.target_ip != null
      error_message = "target_ip must be set when target_type is lightsail."
    }
  }
}
