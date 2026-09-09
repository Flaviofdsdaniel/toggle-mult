module "dynamodb_table" {
  source  = "terraform-aws-modules/dynamodb-table/aws"
  version = "5.5.1"

  name     = "ToggleMasterAnalytics"
  hash_key = "id"

  billing_mode = "PAY_PER_REQUEST"

  attributes = [
    {
      name = "id"
      type = "S"
    }
  ]
  tags = {
    Name        = "DynamoDBToggleMasterAnalytics"
    Environment = "ToggleMaster"
  }
}