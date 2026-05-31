package crdt

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func RunUpdateBus(artifactsDir, logsDir, addr string) error {
	return shared.RunCRDTUpdateBus(artifactsDir, logsDir, addr)
}
