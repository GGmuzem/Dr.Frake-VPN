import { describe, expect, it } from "vitest";
import { defaultSiteConfig, normalizeSiteConfig, supportChannels, visiblePlanIds } from "../lib/site-config";

describe("site config", () => {
  it("uses FBLink VPN branding only", () => {
    expect(defaultSiteConfig.brand).toBe("FBLink VPN");
  });

  it("exposes only Premium and VIP plans with one and three month periods", () => {
    expect(visiblePlanIds(defaultSiteConfig)).toEqual(["basic", "basic_3m", "vip", "vip_3m"]);
  });

  it("fills missing plan metadata from defaults for older backend config", () => {
    const config = normalizeSiteConfig({
      brand: "FBLink VPN",
      plans: [
        {
          code: "premium",
          title: "Premium",
          periods: [
            { id: "basic", label: "1 месяц", duration_days: 30, amount: 199, currency: "RUB" },
            { id: "basic_3m", label: "3 месяца", duration_days: 90, amount: 505, currency: "RUB" },
          ],
        },
        {
          code: "vip",
          title: "VIP",
          periods: [
            { id: "vip", label: "1 месяц", duration_days: 30, amount: 399, currency: "RUB" },
            { id: "vip_3m", label: "3 месяца", duration_days: 90, amount: 1015, currency: "RUB" },
          ],
        },
      ],
    });

    expect(config.plans[0].description).toBe(defaultSiteConfig.plans[0].description);
    expect(config.plans[0].features).toEqual(defaultSiteConfig.plans[0].features);
    expect(visiblePlanIds(config)).toEqual(["basic", "basic_3m", "vip", "vip_3m"]);
  });

  it("uses the current support email and exposes messenger channels", () => {
    expect(defaultSiteConfig.support.email).toBe("fbapps.help@yandex.ru");
    expect(supportChannels(defaultSiteConfig).map((channel) => channel.id)).toEqual(["telegram", "whatsapp", "max"]);
  });
});
