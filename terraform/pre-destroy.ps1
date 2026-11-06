[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ClusterName,

    [Parameter(Mandatory = $true)]
    [string]$VpcId,

    [Parameter(Mandatory = $true)]
    [string]$Region
)

$ErrorActionPreference = "Stop"

function Assert-NativeCommand {
    param([Parameter(Mandatory = $true)][string]$Name)

    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE. Terraform destroy was stopped before removing EKS or the VPC."
    }
}

# A previous destroy may have removed EKS before failing on a leftover AWS
# load balancer. In that retry case the controller cannot run anymore.
$clusterStatus = & aws eks describe-cluster --region $Region --name $ClusterName `
    --query "cluster.status" --output text 2>$null
$clusterLookupExitCode = $LASTEXITCODE

if ($clusterLookupExitCode -ne 0 -or -not $clusterStatus -or $clusterStatus -eq "None") {
    $loadBalancerOutput = & aws elbv2 describe-load-balancers --region $Region `
        --query "LoadBalancers[?VpcId=='$VpcId'].LoadBalancerArn" --output text
    Assert-NativeCommand -Name "aws elbv2 describe-load-balancers"
    $remainingLoadBalancers = @(
        $loadBalancerOutput -split '\s+' |
            Where-Object { $_ -and $_ -ne "None" }
    )

    $eniJson = & aws ec2 describe-network-interfaces `
        --filters "Name=vpc-id,Values=$VpcId" --region $Region --output json
    Assert-NativeCommand -Name "aws ec2 describe-network-interfaces"
    $eniResponse = $eniJson | ConvertFrom-Json
    $remainingElbEnis = @(
        $eniResponse.NetworkInterfaces |
            Where-Object { $_.Description -like "ELB *" }
    )

    if ($remainingLoadBalancers.Count -eq 0 -and $remainingElbEnis.Count -eq 0) {
        Write-Host "EKS is already absent and no ELB resources remain; Terraform can continue the cleanup."
        exit 0
    }

    $lbList = $remainingLoadBalancers -join ", "
    $eniList = ($remainingElbEnis | ForEach-Object { $_.NetworkInterfaceId }) -join ", "
    throw "EKS is already absent, so the AWS Load Balancer Controller cannot clean up remaining resources. Load balancers: [$lbList]. ENIs: [$eniList]. Delete the listed load balancer(s) in AWS, then retry terraform destroy."
}

Write-Host "Updating kubeconfig for EKS cluster '$ClusterName'..."
& aws eks update-kubeconfig --region $Region --name $ClusterName
Assert-NativeCommand -Name "aws eks update-kubeconfig"

Write-Host "Checking Kubernetes API access..."
& kubectl get nodes --request-timeout=30s | Out-Null
Assert-NativeCommand -Name "kubectl get nodes"

# Deleting the ApplicationSet cascades to its generated Applications and their
# managed resources. preserveResourcesOnDeletion is explicitly false in the
# ApplicationSet manifest so the ALB Ingress is removed while its controller is up.
$applicationSetCrd = & kubectl get crd applicationsets.argoproj.io --ignore-not-found -o name
Assert-NativeCommand -Name "kubectl get ApplicationSet CRD"

if ($applicationSetCrd) {
    $applicationSet = & kubectl get applicationset.argoproj.io togglemaster-services `
        --namespace argocd --ignore-not-found -o name
    Assert-NativeCommand -Name "kubectl get ApplicationSet"

    if ($applicationSet) {
        Write-Host "Deleting the ApplicationSet and its generated applications..."
        & kubectl delete applicationset.argoproj.io togglemaster-services `
            --namespace argocd --cascade=foreground --wait=true --timeout=15m
        Assert-NativeCommand -Name "kubectl delete ApplicationSet"
    }
}

# The direct delete is an idempotent safeguard in case the Ingress was not
# included in the ApplicationSet's generated resources.
Write-Host "Deleting the ToggleMaster Ingress..."
& kubectl delete ingress togglemaster-ingress --namespace toggle-mult `
    --ignore-not-found=true --wait=true --timeout=15m
Assert-NativeCommand -Name "kubectl delete Ingress"

$deadline = (Get-Date).AddMinutes(15)
$pendingLoadBalancers = @()
$pendingLoadBalancerEnis = @()

do {
    $loadBalancerOutput = & aws elbv2 describe-load-balancers --region $Region `
        --query "LoadBalancers[?VpcId=='$VpcId'].LoadBalancerArn" --output text
    Assert-NativeCommand -Name "aws elbv2 describe-load-balancers"
    $pendingLoadBalancers = @(
        $loadBalancerOutput -split '\s+' |
            Where-Object { $_ -and $_ -ne "None" }
    )

    $eniJson = & aws ec2 describe-network-interfaces `
        --filters "Name=vpc-id,Values=$VpcId" --region $Region --output json
    Assert-NativeCommand -Name "aws ec2 describe-network-interfaces"
    $eniResponse = $eniJson | ConvertFrom-Json
    $pendingLoadBalancerEnis = @(
        $eniResponse.NetworkInterfaces |
            Where-Object { $_.Description -like "ELB *" }
    )

    if ($pendingLoadBalancers.Count -eq 0 -and $pendingLoadBalancerEnis.Count -eq 0) {
        break
    }

    if ((Get-Date) -ge $deadline) {
        $lbList = $pendingLoadBalancers -join ", "
        $eniList = ($pendingLoadBalancerEnis | ForEach-Object { $_.NetworkInterfaceId }) -join ", "
        throw "Timed out waiting for load balancers/ENIs to disappear. Load balancers: [$lbList]. ENIs: [$eniList]. Terraform kept EKS and the VPC so cleanup can be retried."
    }

    Write-Host "Waiting for load balancers and their ENIs to be removed..."
    Start-Sleep -Seconds 15
} while ($true)

Write-Host "GitOps resources and load balancers are cleaned up; Terraform can now destroy EKS and the VPC."
