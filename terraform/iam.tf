# EKS POD IDENTITY TRUST

data "aws_iam_policy_document" "pod_identity_trust" {

  statement {
    effect = "Allow"

    principals {
      type = "Service"

      identifiers = [
        "pods.eks.amazonaws.com"
      ]
    }

    actions = [
      "sts:AssumeRole",
      "sts:TagSession"
    ]
  }
}

# EVALUATION SERVICE

resource "aws_iam_role" "evaluation" {
  name = "ToggleMaster-Evaluation-Role"

  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json

}

data "aws_iam_policy_document" "evaluation" {

  statement {
    effect = "Allow"

    actions = [
      "sqs:SendMessage",
      "sqs:GetQueueUrl"
    ]

    resources = [
      module.sqs.queue_arn
    ]
  }
}

resource "aws_iam_policy" "evaluation" {
  name = "ToggleMaster-Evaluation-Policy"

  policy = data.aws_iam_policy_document.evaluation.json

}

resource "aws_iam_role_policy_attachment" "evaluation" {
  role       = aws_iam_role.evaluation.name
  policy_arn = aws_iam_policy.evaluation.arn
}

# ANALYTICS SERVICE

resource "aws_iam_role" "analytics" {
  name = "ToggleMaster-Analytics-Role"

  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json

}

data "aws_iam_policy_document" "analytics" {

  statement {
    effect = "Allow"

    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:ChangeMessageVisibility",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl"
    ]

    resources = [
      module.sqs.queue_arn
    ]
  }

  statement {
    effect = "Allow"

    actions = [
      "dynamodb:PutItem"
    ]

    resources = [
      module.dynamodb_table.dynamodb_table_arn
    ]
  }
}

resource "aws_iam_policy" "analytics" {
  name = "ToggleMaster-Analytics-Policy"

  policy = data.aws_iam_policy_document.analytics.json

}

resource "aws_iam_role_policy_attachment" "analytics" {
  role       = aws_iam_role.analytics.name
  policy_arn = aws_iam_policy.analytics.arn
}

# POD IDENTITY ASSOCIATIONS

resource "aws_eks_pod_identity_association" "evaluation" {
  cluster_name = module.eks.cluster_name

  namespace       = "toggle-mult"
  service_account = "evaluation-service-sa"

  role_arn = aws_iam_role.evaluation.arn
}

resource "aws_eks_pod_identity_association" "analytics" {
  cluster_name = module.eks.cluster_name

  namespace       = "toggle-mult"
  service_account = "analytics-service-sa"

  role_arn = aws_iam_role.analytics.arn
}

# KEDA OPERATOR

resource "aws_iam_role" "keda" {
  name = "ToggleMaster-KEDA-Role"

  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

data "aws_iam_policy_document" "keda" {
  statement {
    effect = "Allow"

    actions = [
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl"
    ]

    resources = [
      module.sqs.queue_arn
    ]
  }
}

resource "aws_iam_policy" "keda" {
  name = "ToggleMaster-KEDA-Policy"

  policy = data.aws_iam_policy_document.keda.json
}

resource "aws_iam_role_policy_attachment" "keda" {
  role       = aws_iam_role.keda.name
  policy_arn = aws_iam_policy.keda.arn
}

resource "aws_eks_pod_identity_association" "keda" {
  cluster_name = module.eks.cluster_name

  namespace       = "keda"
  service_account = "keda-operator"

  role_arn = aws_iam_role.keda.arn

  depends_on = [
    helm_release.keda
  ]
}