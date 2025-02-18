// Copyright 2020 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2restart

import (
	"context"
	"encoding/json"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/chaos-mesh/chaos-mesh/controllers/common"
)

type Impl struct {
	client.Client

	Log logr.Logger
}

var _ common.ChaosImpl = (*Impl)(nil)

func (impl *Impl) Apply(ctx context.Context, index int, records []*v1alpha1.Record, obj v1alpha1.InnerObject) (v1alpha1.Phase, error) {
	awschaos := obj.(*v1alpha1.AWSChaos)

	var selected v1alpha1.AWSSelector
	json.Unmarshal([]byte(records[index].Id), &selected)
	opts := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(selected.AWSRegion),
	}

	if awschaos.Spec.SecretName != nil {
		secret := &v1.Secret{}
		err := impl.Client.Get(ctx, types.NamespacedName{
			Name:      *awschaos.Spec.SecretName,
			Namespace: awschaos.Namespace,
		}, secret)
		if err != nil {
			impl.Log.Error(err, "fail to get cloud secret")
			return v1alpha1.NotInjected, err
		}
		opts = append(opts, awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			string(secret.Data["aws_access_key_id"]),
			string(secret.Data["aws_secret_access_key"]),
			"",
		)))
	}
	cfg, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		impl.Log.Error(err, "unable to load aws SDK config")
		return v1alpha1.NotInjected, err
	}
	ec2client := ec2.NewFromConfig(cfg)

	_, err = ec2client.RebootInstances(context.TODO(), &ec2.RebootInstancesInput{
		InstanceIds: []string{selected.Ec2Instance},
	})

	if err != nil {
		impl.Log.Error(err, "fail to restart the instance")
		return v1alpha1.NotInjected, err
	}

	return v1alpha1.Injected, nil
}

func (impl *Impl) Recover(_ context.Context, _ int, _ []*v1alpha1.Record, _ v1alpha1.InnerObject) (v1alpha1.Phase, error) {
	return v1alpha1.NotInjected, nil
}

func NewImpl(c client.Client, log logr.Logger) *Impl {
	return &Impl{
		Client: c,
		Log:    log.WithName("ec2restart"),
	}
}
