export type RoutingAction = "proxy" | "direct";
export type RoutingKind = "system" | "custom";

export type RoutingProfile = {
  id: number;
  name: string;
  code: string;
  kind: RoutingKind;
  action: RoutingAction;
  enabled: boolean;
  description: string;
  domains?: string[];
  domain_suffixes?: string[];
  cidrs?: string[];
  already_added?: boolean;
};

export type RoutingRuleCounts = {
  domains: number;
  suffixes: number;
  cidrs: number;
  total: number;
};

export type HappRoutingProfile = {
  Name: "FBLink VPN";
  GlobalProxy: "true";
  RouteOrder: "block-proxy-direct";
  RemoteDNSType: "DoH";
  RemoteDNSDomain: string;
  RemoteDNSIP: string;
  DomesticDNSType: "DoH";
  DomesticDNSDomain: string;
  DomesticDNSIP: string;
  Geoipurl: string;
  Geositeurl: string;
  LastUpdated: string;
  DnsHosts: Record<string, string>;
  DirectSites: string[];
  DirectIp: string[];
  ProxySites: string[];
  ProxyIp: string[];
  BlockSites: string[];
  BlockIp: string[];
  DomainStrategy: "IPIfNonMatch";
  FakeDNS: "false";
  UseChunkFiles: "true";
};

const happDefaultDirectIpRules = [
  "10.0.0.0/8",
  "172.16.0.0/12",
  "192.168.0.0/16",
  "169.254.0.0/16",
  "224.0.0.0/4",
  "255.255.255.255",
];

export function normalizeRuleText(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(/[\s,;]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item) => {
      if (seen.has(item)) return false;
      seen.add(item);
      return true;
    });
}

export function countRoutingRules(profile: Pick<RoutingProfile, "domains" | "domain_suffixes" | "cidrs">): RoutingRuleCounts {
  const domains = profile.domains?.length ?? 0;
  const suffixes = profile.domain_suffixes?.length ?? 0;
  const cidrs = profile.cidrs?.length ?? 0;
  return { domains, suffixes, cidrs, total: domains + suffixes + cidrs };
}

function pluralRu(count: number, one: string, few: string, many: string): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
  return many;
}

export function routingSummary(profile: Pick<RoutingProfile, "domains" | "domain_suffixes" | "cidrs">): string {
  const counts = countRoutingRules(profile);
  if (counts.total === 0) return "Правил нет";

  const parts: string[] = [];
  if (counts.domains > 0) {
    parts.push(`${counts.domains} ${pluralRu(counts.domains, "домен", "домена", "доменов")}`);
  }
  if (counts.suffixes > 0) {
    parts.push(`${counts.suffixes} ${pluralRu(counts.suffixes, "зона", "зоны", "зон")}`);
  }
  if (counts.cidrs > 0) {
    parts.push(`${counts.cidrs} IP`);
  }
  return parts.join(" · ");
}

function appendUnique(target: string[], value: string): void {
  const normalized = value.trim();
  if (normalized && !target.includes(normalized)) {
    target.push(normalized);
  }
}

function appendHappRules(profile: RoutingProfile, sites: string[], ips: string[]): void {
  (profile.domains || []).forEach((domain) => appendUnique(sites, `full:${domain}`));
  (profile.domain_suffixes || []).forEach((suffix) => {
    const normalized = suffix.trim().replace(/^\./, "");
    if (normalized) appendUnique(sites, `domain:${normalized}`);
  });
  (profile.cidrs || []).forEach((cidr) => appendUnique(ips, cidr));
}

export function buildHappRoutingProfile(profiles: RoutingProfile[], nowUnix = Math.floor(Date.now() / 1000)): HappRoutingProfile {
  const directSites: string[] = [];
  const directIp: string[] = [...happDefaultDirectIpRules];
  const proxySites: string[] = [];
  const proxyIp: string[] = [];

  profiles
    .filter((profile) => profile.kind === "custom" && profile.enabled)
    .forEach((profile) => {
      if (profile.action === "proxy") {
        appendHappRules(profile, proxySites, proxyIp);
      } else {
        appendHappRules(profile, directSites, directIp);
      }
    });

  return {
    Name: "FBLink VPN",
    GlobalProxy: "true",
    RouteOrder: "block-proxy-direct",
    RemoteDNSType: "DoH",
    RemoteDNSDomain: "https://cloudflare-dns.com/dns-query",
    RemoteDNSIP: "1.1.1.1",
    DomesticDNSType: "DoH",
    DomesticDNSDomain: "https://dns.google/dns-query",
    DomesticDNSIP: "8.8.8.8",
    Geoipurl: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
    Geositeurl: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
    LastUpdated: String(nowUnix),
    DnsHosts: {
      "cloudflare-dns.com": "1.1.1.1",
      "dns.google": "8.8.8.8",
    },
    DirectSites: directSites,
    DirectIp: directIp,
    ProxySites: proxySites,
    ProxyIp: proxyIp,
    BlockSites: [],
    BlockIp: [],
    DomainStrategy: "IPIfNonMatch",
    FakeDNS: "false",
    UseChunkFiles: "true",
  };
}
