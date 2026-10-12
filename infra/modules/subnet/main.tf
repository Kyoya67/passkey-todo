resource "aws_subnet" "public" {
  for_each = { for index, az in var.availability_zones : az => var.public_cidrs[index] }

  vpc_id                  = var.vpc_id
  availability_zone       = each.key
  cidr_block              = each.value
  map_public_ip_on_launch = true
  tags                    = var.tags
}

resource "aws_subnet" "private" {
  for_each = { for index, az in var.availability_zones : az => var.private_cidrs[index] }

  vpc_id            = var.vpc_id
  availability_zone = each.key
  cidr_block        = each.value
  tags              = var.tags
}

