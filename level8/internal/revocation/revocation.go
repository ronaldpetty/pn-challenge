package revocation

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func RunAuthority(artifactsDir, logsDir, addr string) error {
	return shared.RunRevocationAuthority(artifactsDir, logsDir, addr)
}
