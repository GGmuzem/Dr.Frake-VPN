import { describe, expect, it } from "vitest";
import {
  applyServerFilters,
  hasUnacknowledgedCriticalIncident,
  mergeNotificationEvent,
  preserveSelectedServer,
} from "../lib/admin";

describe("admin panel helpers", () => {
  it("detects unacknowledged critical incidents for the banner", () => {
    expect(
      hasUnacknowledgedCriticalIncident([
        {
          id: 1,
          severity: "critical",
          status: "open",
          title: "VPS недоступен",
          message: "Нет heartbeat",
          created_at: "2026-05-30T12:00:00Z",
          last_seen_at: "2026-05-30T12:00:00Z",
          acknowledged_at: null,
          muted_until: null,
        },
      ]),
    ).toBe(true);
    expect(
      hasUnacknowledgedCriticalIncident([
        {
          id: 1,
          severity: "critical",
          status: "open",
          title: "VPS недоступен",
          message: "Нет heartbeat",
          created_at: "2026-05-30T12:00:00Z",
          last_seen_at: "2026-05-30T12:00:00Z",
          acknowledged_at: "2026-05-30T12:01:00Z",
          muted_until: null,
        },
      ]),
    ).toBe(false);
  });

  it("merges SSE notification events by id", () => {
    const merged = mergeNotificationEvent(
      [
        {
          id: 1,
          severity: "warning",
          status: "open",
          title: "Old",
          message: "Old",
          created_at: "2026-05-30T12:00:00Z",
          last_seen_at: "2026-05-30T12:00:00Z",
          acknowledged_at: null,
          muted_until: null,
        },
      ],
      [
        {
          id: 1,
          severity: "critical",
          status: "open",
          title: "New",
          message: "New",
          created_at: "2026-05-30T12:00:00Z",
          last_seen_at: "2026-05-30T12:05:00Z",
          acknowledged_at: null,
          muted_until: null,
        },
      ],
    );
    expect(merged).toHaveLength(1);
    expect(merged[0].title).toBe("New");
  });

  it("preserves selected server when filters still include it", () => {
    const servers = [
      { id: 1, name: "Amsterdam", region: "NL", endpoint: "ams.example.com", active: true, is_vip_only: false, agent_mode: "agent", agent_heartbeat_stale: false, utilization: 35 },
      { id: 2, name: "Paris", region: "FR", endpoint: "par.example.com", active: true, is_vip_only: true, agent_mode: "agent", agent_heartbeat_stale: true, utilization: 92 },
    ];
    const filtered = applyServerFilters(servers, { query: "paris", status: "stale", region: "", vipOnly: "all" });
    expect(filtered.map((server) => server.id)).toEqual([2]);
    expect(preserveSelectedServer(2, filtered)).toBe(2);
    expect(preserveSelectedServer(1, filtered)).toBe(2);
  });
});
