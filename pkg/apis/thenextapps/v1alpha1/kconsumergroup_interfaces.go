package v1alpha1

// KConsumerGroupOperator interface contains functions for operator
type KConsumerGroupOperator interface {
	CreateConsumerDeployment() (n int, err error)
	UpdateConsumerDeployment() (n int, err error)
	DeleteConsumerDeployment() (n int, err error)
	CreatePrometheusRule() (n int, err error)
	CreateServiceMonitor() (n int, err error)
}
