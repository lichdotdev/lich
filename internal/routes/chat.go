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
	rpc.Handle(d, api.ChatRunShell, s.RunShell)
	rpc.Handle(d, api.ChatNew, s.New)
	rpc.Handle(d, api.ChatMCP, s.MCP)
	rpc.Handle(d, api.ChatMCPAct, s.MCPAct)
	rpc.Handle(d, api.ChatSkills, s.Skills)
	rpc.Handle(d, api.ChatSetSkill, s.SetSkill)
	rpc.Handle(d, api.ChatPlugins, s.Plugins)
	rpc.Handle(d, api.ChatPluginAct, s.PluginAct)
	rpc.Handle(d, api.ChatPermissions, s.Permissions)
	rpc.Handle(d, api.ChatPermissionAct, s.PermissionAct)
	rpc.Handle(d, api.ChatHooks, s.Hooks)
	rpc.Handle(d, api.ChatHookAct, s.HookAct)
	rpc.Handle(d, api.ChatShells, s.Shells)
	rpc.Handle(d, api.ChatShellOutput, s.ShellOutput)
	rpc.Handle(d, api.ChatStopShell, s.StopShell)
	rpc.Handle(d, api.ChatBackground, s.Background)
}
