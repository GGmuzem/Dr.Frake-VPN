import { describe, expect, it } from "vitest";
import {
  buildHappRoutingProfile,
  countRoutingRules,
  normalizeRuleText,
  routingSummary,
} from "../lib/routing-profiles";

describe("routing profile helpers", () => {
  it("normalizes multiline routing input without duplicates", () => {
    expect(normalizeRuleText("youtube.com, twitch.tv\n youtube.com  .ru")).toEqual([
      "youtube.com",
      "twitch.tv",
      ".ru",
    ]);
  });

  it("counts domains, suffixes and cidrs separately", () => {
    expect(
      countRoutingRules({
        domains: ["youtube.com"],
        domain_suffixes: [".googlevideo.com", ".ytimg.com"],
        cidrs: ["10.0.0.0/8"],
      }),
    ).toEqual({ domains: 1, suffixes: 2, cidrs: 1, total: 4 });
    expect(routingSummary({ domains: ["youtube.com"], domain_suffixes: [".ru"], cidrs: [] })).toBe(
      "1 домен · 1 зона",
    );
  });

  it("builds Happ routing payload from enabled custom profiles only", () => {
    const payload = buildHappRoutingProfile(
      [
        {
          id: 1,
          name: "AI",
          code: "",
          kind: "custom",
          action: "proxy",
          enabled: true,
          description: "",
          domains: ["chatgpt.com"],
          domain_suffixes: [".openai.com"],
          cidrs: [],
        },
        {
          id: 2,
          name: "RU",
          code: "",
          kind: "custom",
          action: "direct",
          enabled: true,
          description: "",
          domains: ["gosuslugi.ru"],
          domain_suffixes: [".ru"],
          cidrs: ["10.0.0.0/8"],
        },
        {
          id: 3,
          name: "Disabled",
          code: "",
          kind: "custom",
          action: "proxy",
          enabled: false,
          description: "",
          domains: ["disabled.example"],
          domain_suffixes: [],
          cidrs: [],
        },
      ],
      123,
    );

    expect(payload.Name).toBe("FBLink VPN");
    expect(payload.RouteOrder).toBe("block-proxy-direct");
    expect(payload.UseChunkFiles).toBe("true");
    expect(payload.ProxySites).toEqual(["full:chatgpt.com", "domain:openai.com"]);
    expect(payload.DirectSites).toEqual(["full:gosuslugi.ru", "domain:ru"]);
    expect(payload.DirectIp).toEqual([
      "10.0.0.0/8",
      "172.16.0.0/12",
      "192.168.0.0/16",
      "169.254.0.0/16",
      "224.0.0.0/4",
      "255.255.255.255",
    ]);
    expect(payload.ProxySites).not.toContain("full:disabled.example");
    expect(payload.LastUpdated).toBe("123");
  });
});
