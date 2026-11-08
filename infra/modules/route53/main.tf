resource "aws_route53_zone" "this" {
  name = var.zone_name

  tags = var.tags
}

resource "aws_route53_record" "api" {
  zone_id = aws_route53_zone.this.zone_id
  name    = var.record_name
  type    = "A"

  records = var.target == "lightsail" ? [var.lightsail_ip] : null
  ttl     = var.target == "lightsail" ? 300 : null

  dynamic "alias" {
    for_each = var.target == "alb" ? [1] : []

    content {
      name                   = var.alb_dns_name
      zone_id                = var.alb_zone_id
      evaluate_target_health = true
    }
  }
}

moved {
  from = aws_route53_record.alb
  to   = aws_route53_record.api
}
