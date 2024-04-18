package config

type DeployConfig struct {
	RoleARN string `json:"roleArn,omitempty" yaml:"roleArn,omitempty"`
	Region  string `json:"region,omitempty" yaml:"region,omitempty"`
}

func NewDeployConfig() DeployConfig {
	return DeployConfig{
		Region: "us-west-2",
	}
}
