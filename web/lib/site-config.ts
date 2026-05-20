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
  downloads: Record<"android" | "windows" | "macos" | "linux" | "happ", string>;
  support: {
    email: string;
    telegram: string;
  };
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
      features: ["Безлимитный трафик", "Все основные платформы", "iOS через Happ"],
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
    android: "https://srv.frakebit.com/download/android",
    windows: "https://srv.frakebit.com/download/windows",
    macos: "https://srv.frakebit.com/download/macos",
    linux: "https://srv.frakebit.com/download/linux",
    happ: "https://apps.apple.com/search?term=happ%20proxy",
  },
  support: {
    email: "support@frakebit.com",
    telegram: "https://t.me/fblinkvpn_support",
  },
};

export function visiblePlanIds(config: SiteConfig): PlanId[] {
  return config.plans.flatMap((plan) => plan.periods.map((period) => period.id));
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
    const data = (await response.json()) as SiteConfig;
    return {
      ...defaultSiteConfig,
      ...data,
      downloads: { ...defaultSiteConfig.downloads, ...data.downloads },
      support: { ...defaultSiteConfig.support, ...data.support },
    };
  } catch {
    return defaultSiteConfig;
  }
}
