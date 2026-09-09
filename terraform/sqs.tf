module "sqs" {
  source = "terraform-aws-modules/sqs/aws"

  name = "ToggleMasterAnalyticsQueue"

  # Fila normal, não FIFO
  fifo_queue = false

  # Tempo máximo que uma mensagem pode ficar na fila
  message_retention_seconds = 60

  # Tempo que a mensagem fica invisível
  # depois que um consumidor recebe
  visibility_timeout_seconds = 30

  # Long polling
  receive_wait_time_seconds = 20


}