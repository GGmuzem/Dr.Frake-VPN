export type PlanId = "basic" | "basic_3m" | "vip" | "vip_3m";

export type PlanPeriod = {
  id: PlanId;
  label: string;
  duration_days: number;
  amount: number;
  currency: "RUB";
};

export type Plan = {
  code: "premium" | "vip";
  title: "Premium" | "VIP";
  description: string;
  periods: PlanPeriod[];
  features: string[];
};

export type SiteConfig = {
  brand: "FBLink VPN";
  plans: Plan[];
  downloads: Record<"android" | "windows" | "macos" | "linux" | "happ" | "androidtv", string>;
  support: {
    email: string;
    telegram: string;
  };
};

type SiteConfigInput = Partial<Omit<SiteConfig, "plans">> & {
  plans?: Array<Partial<Omit<Plan, "periods">> & {
    code?: Plan["code"];
    periods?: Array<Partial<PlanPeriod> & { id?: PlanId }>;
  }>;
};

export const defaultSiteConfig: SiteConfig = {
  brand: "FBLink VPN",
  plans: [
    {
      code: "premium",
      title: "Premium",
      description: "Быстрый защищенный доступ для ежедневной работы.",
      periods: [
        { id: "basic", label: "1 месяц", duration_days: 30, amount: 199, currency: "RUB" },
        { id: "basic_3m", label: "3 месяца", duration_days: 90, amount: 505, currency: "RUB" },
      ],
      features: ["Безлимитный трафик", "Все основные платформы", "Быстрое подключение"],
    },
    {
      code: "vip",
      title: "VIP",
      description: "Приоритетная сеть, Xray и расширенные функции приватности.",
      periods: [
        { id: "vip", label: "1 месяц", duration_days: 30, amount: 399, currency: "RUB" },
        { id: "vip_3m", label: "3 месяца", duration_days: 90, amount: 1015, currency: "RUB" },
      ],
      features: ["VLESS/Xray Reality", "VIP-серверы", "AdBlock DNS"],
    },
  ],
  downloads: {
    android: "https://fblink-sc.com/download/android",
    windows: "https://fblink-sc.com/download/windows",
    macos: "https://fblink-sc.com/download/macos",
    linux: "https://fblink-sc.com/download/linux",
    happ: "https://apps.apple.com/search?term=happ%20proxy",
    androidtv: "https://fblink-sc.com/download/androidtv",
  },
  support: {
    email: "support@frakebit.com",
    telegram: "https://t.me/+79966732628",
  },
};

export function visiblePlanIds(config: SiteConfig): PlanId[] {
  return config.plans.flatMap((plan) => plan.periods.map((period) => period.id));
}

export function normalizeSiteConfig(data: SiteConfigInput | null | undefined): SiteConfig {
  const incomingPlans = Array.isArray(data?.plans) ? data.plans : [];
  const plans = defaultSiteConfig.plans.map((fallbackPlan) => {
    const incomingPlan = incomingPlans.find((plan) => plan?.code === fallbackPlan.code);
    const incomingPeriods = Array.isArray(incomingPlan?.periods) ? incomingPlan.periods : [];
    return {
      ...fallbackPlan,
      ...incomingPlan,
      description: incomingPlan?.description || fallbackPlan.description,
      features: Array.isArray(incomingPlan?.features) && incomingPlan.features.length > 0 ? incomingPlan.features : fallbackPlan.features,
      periods: fallbackPlan.periods.map((fallbackPeriod) => {
        const incomingPeriod = incomingPeriods.find((period) => period?.id === fallbackPeriod.id);
        return { ...fallbackPeriod, ...incomingPeriod };
      }),
    } as Plan;
  });

  return {
    ...defaultSiteConfig,
    ...data,
    brand: "FBLink VPN",
    plans,
    downloads: { ...defaultSiteConfig.downloads, ...data?.downloads },
    support: { ...defaultSiteConfig.support, ...data?.support },
  };
}

export function formatRub(amount: number): string {
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(amount);
}

export async function loadSiteConfig(): Promise<SiteConfig> {
  const base = process.env.NEXT_PUBLIC_API_BASE_URL ?? process.env.BACKEND_API_URL;
  if (!base) {
    return defaultSiteConfig;
  }

  try {
    const response = await fetch(`${base.replace(/\/$/, "")}/api/v1/web/config`, {
      next: { revalidate: 60 },
    });
    if (!response.ok) {
      return defaultSiteConfig;
    }
    const data = (await response.json()) as SiteConfigInput;
    return normalizeSiteConfig(data);
  } catch {
    return defaultSiteConfig;
  }
}
