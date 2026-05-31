package index

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func Run(artifactsDir, logsDir, indexID, addr string) error {
	return shared.RunIndex(artifactsDir, logsDir, indexID, addr)
}
