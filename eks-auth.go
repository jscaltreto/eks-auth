/*
MIT License

Copyright (c) 2022 Jake Scaltreto

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	K8S_AWS_ID_HEADER = "x-k8s-aws-id"
	TOKEN_PREFIX      = "k8s-aws-v1."

	KIND        = "ExecCredential"
	API_VERSION = "client.authentication.k8s.io/v1beta1"

	EXPIRE_PARAM       = "X-Amz-Expires"
	EXPIRE_PARAM_VALUE = "60"
)

type ExecCredential struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Status     map[string]string `json:"status"`
}

func main() {
	ctx := context.Background()

	var clusterName, roleArn string
	flag.StringVar(&clusterName, "cluster-name", "", "Name of the EKS cluster")
	flag.StringVar(&roleArn, "role-arn", "", "Assume this role for credentials")
	flag.Parse()

	if clusterName == "" {
		panic("cluster-name is required!")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config %v", err)
	}

	client := sts.NewFromConfig(cfg)

	if roleArn != "" {
		creds := stscreds.NewAssumeRoleProvider(client, roleArn)
		cfg.Credentials = aws.NewCredentialsCache(creds)
		client = sts.NewFromConfig(cfg)
	}

	token, err := getToken(ctx, client, clusterName)
	if err != nil {
		log.Fatalf("Failed to fetch STS token: %v", err)
	}

	auth, err := getExecAuth(token)
	if err != nil {
		log.Fatalf("Failed to generate ExecCredential: %v", err)
	}

	fmt.Println(auth)
}

func getExecAuth(token string) (string, error) {
	execAuth := ExecCredential{
		ApiVersion: API_VERSION,
		Kind:       KIND,
		Status:     map[string]string{"token": token},
	}
	encoded, err := json.Marshal(execAuth)
	return string(encoded), err
}

func getToken(ctx context.Context, client *sts.Client, clusterName string) (string, error) {
	presignSts := sts.NewPresignClient(client)
	req, err := presignSts.PresignGetCallerIdentity(
		ctx,
		&sts.GetCallerIdentityInput{},
		func(pso *sts.PresignOptions) {
			pso.ClientOptions = append(pso.ClientOptions, sts.WithAPIOptions(
				addEksHeader(clusterName),
			))
		},
	)
	if err != nil {
		return "", err
	}

	return TOKEN_PREFIX + base64.URLEncoding.
		WithPadding(base64.NoPadding).
		EncodeToString([]byte(req.URL)), nil
}

func addEksHeader(cluster string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(middleware.BuildMiddlewareFunc("AddEKSId", func(
			ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
		) (middleware.BuildOutput, middleware.Metadata, error) {
			switch req := in.Request.(type) {
			case *smithyhttp.Request:
				query := req.URL.Query()
				query.Add(EXPIRE_PARAM, EXPIRE_PARAM_VALUE)
				req.URL.RawQuery = query.Encode()

				req.Header.Add(K8S_AWS_ID_HEADER, cluster)
			}
			return next.HandleBuild(ctx, in)
		}), middleware.Before)
	}
}
