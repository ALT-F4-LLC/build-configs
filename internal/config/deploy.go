package config

import (
	"encoding/json"
	"fmt"
)

const (
	DefaultEnvironment = "unspecified"
	DefaultAccount     = "677459762413"
	DefaultRegion      = "us-west-2"
)

type DeployConfig struct {
	// Environment is the name of the environment to deploy to.
	Environment string `json:"environment" yaml:"environment"`

	// Account is the AWS account ID used to deploy to this environment.
	Account string `json:"account,omitempty" yaml:"account,omitempty"`

	// RoleARN is the ARN of the role used to deploy to this environment.
	RoleARN string `json:"roleArn,omitempty" yaml:"roleArn,omitempty"`

	// Region is the AWS region this environment will be deployed into.
	Region string `json:"region,omitempty" yaml:"region,omitempty"`

	// If is the value of the `if` field for the GitHub Actions job that will
	// deploy this application.
	If string `json:"if,omitempty" yaml:"if,omitempty"`

	// GithubEnvironment is the GitHub environment the deploy job runs in; empty
	// leaves the job without an `environment:` key. A role that trusts only an
	// environment's OIDC subject needs this set.
	GithubEnvironment string `json:"githubEnvironment,omitempty" yaml:"githubEnvironment,omitempty"`
}

// UnmarshalJSON unmarshals the JSON blob while adding default values
// contextually
func (c *DeployConfig) UnmarshalJSON(data []byte) error {
	type Alias DeployConfig
	deploy := Alias{
		Environment: DefaultEnvironment,
		Account:     DefaultAccount,
		Region:      DefaultRegion,
	}

	if err := json.Unmarshal(data, &deploy); err != nil {
		return err
	}

	// Build the default role from the configured environment and account, so
	// an entry that sets `environment: dev` and omits roleArn gets the -dev
	// role instead of one for the placeholder environment.
	if deploy.RoleARN == "" {
		deploy.RoleARN = fmt.Sprintf(
			"arn:aws:iam::%s:role/altf4llc-gha-%s-deploy-%s",
			deploy.Account,
			Cfg.Name,
			deploy.Environment,
		)
	}
	*c = DeployConfig(deploy)

	return nil
}
