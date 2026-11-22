//go:build !windows

package chat

import "encoding/json"

// fakeProposal is ExitPlanMode's input as Claude Code writes it, in its
// tool_use and its can_use_tool alike, once the plan is in its file.
var fakeProposal = map[string]any{"plan": "# Plan\n\n1. Create hello.txt", "planFilePath": "/home/u/.claude/plans/p.md"}

// claudeExitPlan asks to start on a plan the way Claude Code does (measured on
// 2.1.330): the tool_use, then its can_use_tool. A setMode in the allow
// switches the mode, which system/status reports; a deny keeps plan mode. It
// writes back what lich answered.
func (f *fake) claudeExitPlan(msg func(string, map[string]any)) {
	msg("mp", map[string]any{"type": "tool_use", "id": "toolu_p", "name": "ExitPlanMode", "input": fakeProposal})
	f.send(map[string]any{"type": "control_request", "request_id": "r4", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "ExitPlanMode", "input": fakeProposal, "tool_use_id": "toolu_p", "requires_user_interaction": true,
	}})
	response, _ := f.next()["response"].(map[string]any)
	behavior, _ := response["response"].(map[string]any)
	if str(behavior, "behavior") != "allow" {
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_p", "is_error": true, "content": str(behavior, "message")}}}})
		msg("mp2", map[string]any{"type": "text", "text": "kept:" + str(behavior, "message")})
		f.claudeResult(false, "completed", "")
		return
	}
	perms, _ := behavior["updatedPermissions"].([]any)
	for _, p := range perms {
		if p, _ := p.(map[string]any); str(p, "type") == "setMode" {
			f.send(map[string]any{"type": "system", "subtype": "status", "permissionMode": str(p, "mode")})
		}
	}
	f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_p",
		"content": "User has approved your plan. You can now start coding."}}}, "tool_use_result": map[string]any{"plan": fakeProposal["plan"], "isAgent": false}})
	in, _ := json.Marshal(behavior["updatedInput"])
	msg("mp2", map[string]any{"type": "text", "text": "approved:" + string(in)})
	f.claudeResult(false, "completed", "")
}
