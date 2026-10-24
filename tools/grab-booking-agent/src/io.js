// How the agent talks to people and to OneSystem.

import readline from "node:readline/promises";

// PoC: staff answer in the terminal next to the browser window.
// Later: push to the OneSystem handoff screen and wait for a reply.
export class TerminalHuman {
  async ask(question) {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    try {
      process.stdout.write("\x07"); // bell, so staff notice
      return await rl.question(`\n[CẦN NHÂN VIÊN] ${question}\n> `);
    } finally {
      rl.close();
    }
  }
}

// Scripted answers, for tests.
export class ScriptedHuman {
  constructor(answers = []) {
    this.answers = [...answers];
    this.questions = [];
  }

  async ask(question) {
    this.questions.push(question);
    return this.answers.shift() ?? "n";
  }
}

// Posts the outcome to OneSystem when ONESYSTEM_URL is set, otherwise only logs.
// Endpoint is the planned POST /api/v1/shipments/:id/agent-events
// (business_documents/development/shipment_web_order_research.md §3.7).
export class OneSystemReporter {
  constructor(baseUrl = process.env.ONESYSTEM_URL) {
    this.baseUrl = baseUrl?.replace(/\/$/, "");
  }

  async report(result) {
    if (!this.baseUrl) return;
    const url = `${this.baseUrl}/api/v1/shipments/${encodeURIComponent(result.shipment_id)}/agent-events`;
    const res = await fetch(url, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ provider: "GRAB_EXPRESS", source: "booking-agent", ...result }),
    });
    if (!res.ok) console.error(`OneSystem callback failed: ${res.status} ${await res.text()}`);
  }
}
