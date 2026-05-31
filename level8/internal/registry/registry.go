package registry

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

type Options = shared.RegistryOptions

func RunEnterprise(artifactsDir, logsDir, enterprise, joinIndexURL, addr string) error {
	return shared.RunRegistry(artifactsDir, logsDir, enterprise, joinIndexURL, addr)
}

func RunEnterpriseWithOptions(options shared.RegistryOptions) error {
	return shared.RunRegistryWithOptions(options)
}

func RunPrivateFactsGateway(artifactsDir, logsDir, addr string) error {
	return shared.RunPrivateFactsGateway(artifactsDir, logsDir, addr)
}
