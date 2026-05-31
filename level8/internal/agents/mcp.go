package agents

import "github.com/ronaldpetty/pn-challenge/level8/internal/shared"

func RunMCPServer(logsDir, agentID, tool, addr string) error {
	return shared.RunMCPServer(logsDir, agentID, tool, addr)
}
