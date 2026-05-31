package rotator

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func RunCredentials(artifactsDir, logsDir string) error {
	return shared.RunRotator(artifactsDir, logsDir)
}

func RunIssuerKeys(artifactsDir, logsDir string) error {
	return shared.RunKeyRotator(artifactsDir, logsDir)
}
