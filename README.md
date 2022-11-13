# eks-auth
Standalone program to fetch authentication tokens for AWS EKS Clusters

## Description

This is a standalone program to fetch authentication tokens for AWS EKS Clusters. It functions in much the same way as `aws eks get-token` and supports similar arguments.

Under the hood it uses the AWS Go SDK v2 and will respect certain environment variables such as `AWS_REGION`, `AWS_PROFILE`, and other variables related to authentication.

This is primarily useful for using tools such as [Atlantis](https://www.runatlantis.io/). When running terraform with pre-generated plans (such as in Atlantis), using `exec` for authentication is preferred over fetching a token with the `aws_eks_cluster_auth` data source since the latter will only fetch a short-lived token during the `plan` phase, which may be expired by the time the `apply` is executed.

This was created as a ligher-weight alternative to installing the AWS cli, along with a Python interpreter, in an Atlantis Docker image when the only feature of the CLI being used is EKS authentication.

## Installation

Download the approptiate `tar.gz` file for your platform from the [Releases](https://github.com/jscaltreto/eks-auth/releases/latest) page and extract it. Copy the binary `eks-auth` to a location in your `$PATH`.

## Usage

You must already be authenticated to AWS, or have appropriate environment variables configured to allow for authentication to take place. For details see the documentation for the [AWS SDK for Go V2](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).

To obtain an authentication token, simply execute `eks-auth` passing the name of the cluster to authenticate to:

```
eks-auth --cluster-name my-cluster
```

Additionally, it is possible to assume another IAM role to perform the authentication.

```
eks-auth --cluster-name my-cluster --role-arn arn:aws:iam::123456789012:role/cluster-admin-role
```

Your current role must have the appropriate permissions necessary to assume the role specified by `--role-arn`. This role will also be used to sign the generated token.

## Terraform example

Here is an example of how to initialize the `kubernetes` [Terraform provider](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs) using this program. The `eks-auth` binary must be accessible in the `$PATH` where terraform is run (such as `/usr/bin` in an Atlantis Docker image).

```hcl
data "aws_eks_cluster" "cluster" {
  name = "my-cluster"
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = data.aws_eks_cluster.cluster.certificate_authority[0].data
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["--cluster-name", data.aws_eks_cluster.cluster.id]
    command     = "eks-auth"
  }
}
```

Other providers, such as [Helm](https://registry.terraform.io/providers/hashicorp/helm/latest/docs) similarly support exec authentication.
