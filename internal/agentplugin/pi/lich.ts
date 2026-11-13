// lich-plugin v0.0.0 — shipped with lich
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const provider = "pi";
const pluginVersion = "0.0.0";

export default function (pi: any) {
	const env = process.env;
	const sessionID = () => env.LICH_SESSION_ID;
	let providerSessionID = "";

	async function report(path: string, body: Record<string, unknown>) {
		const port = env.LICH_PORT;
		const token = env.LICH_TOKEN;
		const id = sessionID();
		if (!port || !token || !id) return;
		try {
			await fetch(`http://127.0.0.1:${port}/${path}?token=${encodeURIComponent(token)}`, {
				method: "POST",
				headers: {
					"content-type": "application/json",
					"x-lich-plugin": pluginVersion,
				},
				body: JSON.stringify({ session_id: id, ...body }),
				signal: AbortSignal.timeout(750),
			});
		} catch {
			// Reports must never hold up the agent.
		}
	}

	function reportState(state: string, tool?: string) {
		return report("hook", {
			state,
			tool,
			provider_session_id: providerSessionID,
		});
	}

	function reportModel(model: { id?: string } | undefined) {
		if (!model?.id) return Promise.resolve();
		return report("session-model", { model: model.id });
	}

	const lichBin = env.LICH_BIN;
	if (lichBin && env.LICH_PORT && env.LICH_TOKEN && env.LICH_SESSION_ID) {
		pi.registerMcpServer("lich", { command: lichBin, args: ["mcp"] });
	}

	pi.on("session_start", (_event: unknown, ctx: any) => {
		providerSessionID = ctx.sessionManager.getHeader()?.id ?? "";
		return Promise.all([
			report("session-start", {
				provider_session_id: providerSessionID,
				provider,
			}),
			reportModel(ctx.model),
		]);
	});
	pi.on("model_select", (event: { model: { id: string } }) => reportModel(event.model));
	pi.on("session_info_changed", (event: { name?: string }) => {
		if (event.name) return report("session-title", { title: event.name });
	});
	pi.on("agent_start", () => reportState("busy"));
	pi.on("agent_settled", () => reportState("done"));
	pi.on("tool_execution_start", (event: { toolName: string }) => reportState("busy", event.toolName));
	pi.on("tool_execution_end", (event: { toolName: string }) => {
		if (["write", "edit", "bash"].includes(event.toolName)) {
			return report("session-touched", {});
		}
	});
	pi.on("session_shutdown", (event: { reason: string }) => {
		if (event.reason === "quit") return reportState("idle");
	});
	pi.on("resources_discover", () => ({
		skillPaths: [join(dirname(fileURLToPath(import.meta.url)), "skills")],
	}));
}
