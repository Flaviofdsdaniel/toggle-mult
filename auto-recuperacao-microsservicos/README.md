# Rollback automático GitOps

Quando Prometheus identifica um Deployment cujo rollout passou de `ProgressDeadlineExceeded`, o Alertmanager envia `RollbackVersaoDeDeploy` para este serviço. O serviço lê a imagem atualmente declarada no Deployment e aciona `repository_dispatch` no GitHub. A workflow `.github/workflows/rollback-deployment.yaml` compara essa imagem com a imagem no manifesto GitOps e, se o alerta ainda for atual, publica um commit com a versão anterior distinta. O Argo CD sincroniza o commit e restaura a versão estável.

O rollback só altera o manifesto do serviço permitido. Um alerta atrasado é ignorado se a imagem do Git já tiver avançado. A conta de serviço Kubernetes tem apenas leitura dos Deployments permitidos; ela não altera o cluster.

Notificações repetidas são ignoradas quando o Deployment já terminou de aplicar a imagem desejada e tem todas as réplicas prontas. Isso evita que um alerta ainda não resolvido reverta novamente para a versão que acabou de falhar.

## Credencial necessária

Crie um fine-grained personal access token restrito ao repositório `Flaviofdsdaniel/toggle-mult`, com permissão `Contents: Read and write`, para a API `POST /repos/{owner}/{repo}/dispatches`. Não grave o token no Git.

Armazene o token no AWS Secrets Manager, região `us-east-1`, com o nome `toggle-mult/auto-recuperacao-github` e `SecretString` JSON:

```json
{"GITHUB_TOKEN":"<token>"}
```

O `ExternalSecret` `auto-recuperacao-github`, no namespace `observability`, cria o Secret Kubernetes `auto-recuperacao-credentials`. A automação lê o token como volume, sem reinicialização quando o segredo é atualizado. A prontidão do pod fica indisponível até que o token seja sincronizado.

## Verificação operacional

- A regra `RollbackVersaoDeDeploy` deve aparecer no Prometheus quando um rollout exceder `progressDeadlineSeconds`.
- Os logs do serviço registram o microsserviço e a imagem observada, sem imprimir a credencial.
- A execução da workflow deve criar um commit `gitops: rollback automático ...`.
- O Argo CD deve sincronizar o manifesto restaurado e o Deployment voltar à versão anterior.

Um rollout malsucedido precisa ter uma imagem anterior distinta no histórico do manifesto. Se não houver versão anterior, a workflow falha sem alterar o repositório.
