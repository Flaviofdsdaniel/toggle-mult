terraform {
  backend "s3" {
    bucket       = "togglemaster-terraform-state-togglemaster"
    key          = "ToggleMaster/terraform.tfstate"
    region       = "us-east-1"
    encrypt      = true
    use_lockfile = true

  }
}