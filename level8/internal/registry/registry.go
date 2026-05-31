package registry

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func RunEnterprise(artifactsDir, logsDir, enterprise, joinIndexURL, addr string) error {
	return shared.RunRegistry(artifactsDir, logsDir, enterprise, joinIndexURL, addr)
}

func RunPrivateFactsGateway(artifactsDir, logsDir, addr string) error {
	return shared.RunPrivateFactsGateway(artifactsDir, logsDir, addr)
}
