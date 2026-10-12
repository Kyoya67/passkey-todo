terraform {
  backend "s3" {
    bucket = "ca-techlounge-terraform"
    key    = "main.tfstate"
    region = "ap-northeast-1"
  }
}
