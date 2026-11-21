package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chat"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Chat exposes the chat service as api declares it.
func Chat(d *rpc.Handler, s *chat.Service) {
	rpc.Handle(d, api.ChatKinds, s.Kinds)
	rpc.Handle(d, api.ChatSetView, s.SetView)
	rpc.Handle(d, api.ChatHistory, s.History)
	rpc.Handle(d, api.ChatPrompt, s.Prompt)
	rpc.Handle(d, api.ChatSteer, s.Steer)
	rpc.Handle(d, api.ChatCancel, s.Cancel)
	rpc.Handle(d, api.ChatAnswer, s.Answer)
	rpc.Handle(d, api.ChatReply, s.Reply)
	rpc.Handle(d, api.ChatOptions, s.Options)
	rpc.Handle(d, api.ChatSetOption, s.SetOption)
	rpc.Handle(d, api.ChatCommands, s.Commands)
	rpc.Handle(d, api.ChatNew, s.New)
	rpc.Handle(d, api.ChatMCP, s.MCP)
	rpc.Handle(d, api.ChatMCPAct, s.MCPAct)
	rpc.Handle(d, api.ChatSkills, s.Skills)
	rpc.Handle(d, api.ChatSetSkill, s.SetSkill)
	rpc.Handle(d, api.ChatPlugins, s.Plugins)
	rpc.Handle(d, api.ChatPluginAct, s.PluginAct)
}
