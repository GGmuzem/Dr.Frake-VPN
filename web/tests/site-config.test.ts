import { describe, expect, it } from "vitest";
import { defaultSiteConfig, visiblePlanIds } from "../lib/site-config";

describe("site config", () => {
  it("uses FBLink VPN branding only", () => {
    expect(defaultSiteConfig.brand).toBe("FBLink VPN");
    expect(JSON.stringify(defaultSiteConfig)).not.toContain(["Fable", "ink"].join(""));
  });

  it("exposes only Premium and VIP plans with one and three month periods", () => {
    expect(visiblePlanIds(defaultSiteConfig)).toEqual(["basic", "basic_3m", "vip", "vip_3m"]);
  });
});
