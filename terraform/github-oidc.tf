
# GITHUB OIDC PROVIDER


resource "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"

  client_id_list = [
    "sts.amazonaws.com"
  ]
}

# TRUST POLICY

data "aws_iam_policy_document" "github_actions_trust" {
  statement {
    effect = "Allow"

    actions = [
      "sts:AssumeRoleWithWebIdentity"
    ]

    principals {
      type = "Federated"

      identifiers = [
        aws_iam_openid_connect_provider.github.arn
      ]
    }

    # Token precisa ter sido emitido para AWS STS
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"

      values = [
        "sts.amazonaws.com"
      ]
    }

    # Somente repositorio e somente branch main
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"

      values = [
        "repo:Flaviofdsdaniel@49292830/ToggleMult@1301089250:ref:refs/heads/main"
      ]
    }
  }
}


# IAM ROLE

resource "aws_iam_role" "github_actions_ecr" {
  name = "GitHubActionsECRRole"

  assume_role_policy = data.aws_iam_policy_document.github_actions_trust.json


}


# ECR POLICY DOCUMENT

data "aws_iam_policy_document" "github_actions_ecr" {

  # Necessario para autenticar no ECR
  statement {
    sid    = "ECRLogin"
    effect = "Allow"

    actions = [
      "ecr:GetAuthorizationToken"
    ]

    resources = ["*"]
  }

  # Permite push somente nos ECRs criados pelo projeto
  statement {
    sid    = "ECRPush"
    effect = "Allow"

    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage"
    ]

    resources = [
      for repository in module.ecr :
      repository.repository_arn
    ]
  }
}


# CREATE IAM POLICY

resource "aws_iam_policy" "github_actions_ecr" {
  name        = "GitHubActionsECRPolicy"
  description = "Permite GitHub Actions publicar imagens nos repositorios ECR do ToggleMaster"

  policy = data.aws_iam_policy_document.github_actions_ecr.json


}


# ATTACH POLICY TO ROLE

resource "aws_iam_role_policy_attachment" "github_actions_ecr" {
  role       = aws_iam_role.github_actions_ecr.name
  policy_arn = aws_iam_policy.github_actions_ecr.arn
}