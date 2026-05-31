package consumer

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func Run(artifactsDir, logsDir, indexes string) error {
	return shared.RunConsumer(artifactsDir, logsDir, indexes)
}
