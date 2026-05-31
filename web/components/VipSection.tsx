"use client";

import { useEffect, useState } from "react";
import { ShieldAlert, Zap, Settings2, Plus, Trash2, Edit2, Check, X, Server, Layers, ExternalLink } from "lucide-react";

type RoutingProfile = {
  id: number;
  name: string;
  code: string;
  kind: "system" | "custom";
  action: "proxy" | "direct";
  enabled: boolean;
  description: string;
  domains?: string[];
  domain_suffixes?: string[];
  cidrs?: string[];
  already_added?: boolean;
};

type Session = {
  authenticated: boolean;
  user: { email: string };
  subscription: {
    plan: string;
    status: string;
    expires_at: string;
    auto_renew: boolean;
    vip_ad_block_enabled?: boolean;
  };
};

export function VipSection({ session }: { session: Session }) {
  const [profiles, setProfiles] = useState<RoutingProfile[]>([]);
  const [adBlockEnabled, setAdBlockEnabled] = useState(session.subscription.vip_ad_block_enabled || false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<"custom" | "system">("custom");
  
  // Create / Edit state
  const [isEditing, setIsEditing] = useState<number | null>(null);
  const [editName, setEditName] = useState("");
  const [editDomains, setEditDomains] = useState("");
  const [editAction, setEditAction] = useState<"proxy" | "direct">("proxy");

  const isVip = session.subscription.plan === "vip" || session.subscription.plan === "vip_3m";

  useEffect(() => {
    if (!isVip) return;
    fetchProfiles();
  }, [isVip]);

  const fetchProfiles = () => {
    fetch("/api/routing-profiles")
      .then(res => res.json())
      .then(data => {
        if (data.profiles) setProfiles(data.profiles);
      }).catch(() => {});
  };

  const toggleAdBlock = async () => {
    setError("");
    setLoading(true);
    try {
      const res = await fetch("/api/subscription/ad-block", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: !adBlockEnabled })
      });
      if (!res.ok) throw new Error("Ошибка изменения AdBlock");
      setAdBlockEnabled(!adBlockEnabled);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const toggleProfile = async (profile: RoutingProfile) => {
    setError("");
    setLoading(true);
    try {
      const res = await fetch(`/api/routing-profiles/${profile.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: !profile.enabled })
      });
      if (!res.ok) throw new Error("Ошибка изменения профиля");
      setProfiles(profiles.map(p => p.id === profile.id ? { ...p, enabled: !p.enabled } : p));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const deleteProfile = async (id: number) => {
    if (!confirm("Вы уверены, что хотите удалить это правило?")) return;
    setError("");
    setLoading(true);
    try {
      const res = await fetch(`/api/routing-profiles/${id}`, {
        method: "DELETE"
      });
      if (!res.ok) throw new Error("Ошибка удаления");
      setProfiles(profiles.filter(p => p.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const saveProfile = async () => {
    setError("");
    setLoading(true);
    
    const domainsArray = editDomains.split(/[\s,]+/).filter(Boolean);
    const domains: string[] = [];
    const domain_suffixes: string[] = [];
    const cidrs: string[] = [];

    for (const item of domainsArray) {
      if (/^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/.test(item)) {
        cidrs.push(item);
      } else if (item.startsWith('.')) {
        domain_suffixes.push(item);
      } else {
        domains.push(item);
      }
    }

    const body = {
      name: editName,
      action: editAction,
      domains: domains,
      domain_suffixes: domain_suffixes,
      cidrs: cidrs
    };

    try {
      if (isEditing === -1) {
        // Create
        const res = await fetch("/api/routing-profiles", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        });
        if (!res.ok) throw new Error("Ошибка создания правила");
      } else if (isEditing !== null) {
        // Update
        const res = await fetch(`/api/routing-profiles/${isEditing}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        });
        if (!res.ok) throw new Error("Ошибка обновления правила");
      }
      
      setIsEditing(null);
      fetchProfiles();
      setActiveTab("custom");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const copySystemProfile = async (code: string) => {
    setError("");
    setLoading(true);
    try {
      // Direct fetch to backend proxy for copy
      const tokenRes = await fetch("/api/session");
      if (!tokenRes.ok) throw new Error("Session error");
      const { token } = await tokenRes.json();
      
      const res = await fetch(`/api/v1/me/routing-profiles/system/${code}/copy`, {
        method: "POST",
        headers: { "Authorization": `Bearer ${token}` }
      });
      if (!res.ok) throw new Error("Ошибка добавления системного пресета");
      
      fetchProfiles();
      setActiveTab("custom");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const startCreate = () => {
    setIsEditing(-1);
    setEditName("");
    setEditDomains("");
    setEditAction("proxy");
  };

  const startEdit = (profile: RoutingProfile) => {
    setIsEditing(profile.id);
    setEditName(profile.name);
    
    const allDomains = [
      ...(profile.domains || []),
      ...(profile.domain_suffixes || []),
      ...(profile.cidrs || [])
    ];
    setEditDomains(allDomains.join("\n"));
    
    setEditAction(profile.action || "proxy");
  };
  
  const exportToHapp = () => {
    const customRules = profiles.filter(p => p.kind === 'custom');
    if (customRules.length === 0) {
      alert("Добавьте хотя бы одно правило для экспорта");
      return;
    }
    
    const happRules = customRules.map(p => {
      const allDomains = [
        ...(p.domains || []),
        ...(p.domain_suffixes || []).map(s => s.startsWith('.') ? 'domain:' + s.substring(1) : s)
      ];
      
      const rule: any = {
        type: "field",
        outboundTag: p.action
      };
      
      if (allDomains.length > 0) {
        rule.domain = allDomains;
      }
      
      if (p.cidrs && p.cidrs.length > 0) {
        rule.ip = p.cidrs;
      }
      
      return rule;
    });
    
    const happProfile = {
      name: "FBLink Routing",
      rules: happRules
    };
    
    try {
      const jsonString = JSON.stringify(happProfile);
      const base64Encoded = btoa(unescape(encodeURIComponent(jsonString)));
      const deepLink = `happ://routing/onadd/${base64Encoded}`;
      window.location.href = deepLink;
    } catch (e) {
      setError("Ошибка формирования ссылки");
    }
  };

  if (!isVip) {
    return (
      <div className="panel vip-paywall glass-panel">
        <div className="vip-paywall-content">
          <Zap size={48} className="gold-icon" />
          <h2>Доступно только для VIP</h2>
          <p className="muted">
            Умная маршрутизация и встроенный AdBlock (Pi-Hole) доступны только в тарифе VIP.
          </p>
          <a href="/dashboard#subscription" className="button button-primary gold-button">Обновить до VIP</a>
        </div>
      </div>
    );
  }
  
  const customProfiles = profiles.filter(p => p.kind === 'custom');
  const systemProfiles = profiles.filter(p => p.kind === 'system');

  return (
    <div className="vip-dashboard-content">
      <div className="dashboard-section-head">
        <h1 style={{ display: 'flex', alignItems: 'center', gap: '12px' }}><Zap className="gold-icon" /> VIP Настройки</h1>
        <p className="muted">Управляйте фильтрацией трафика и правилами маршрутизации.</p>
      </div>

      {error && <div className="toast toast-error">{error}</div>}
      
      <div className="panel glass-panel">
        <div className="dashboard-section-head">
          <span className="eyebrow"><ShieldAlert size={12} /> Фильтрация рекламы</span>
          <h2>AdBlock (DNS)</h2>
          <p className="muted">Блокировка рекламы, трекеров и фишинга на уровне серверов.</p>
        </div>
        <div className="toggle-row">
          <strong>AdBlock включен</strong>
          <label className="switch">
            <input type="checkbox" checked={adBlockEnabled} onChange={toggleAdBlock} disabled={loading} />
            <span className="slider"></span>
          </label>
        </div>
      </div>

      <div className="panel glass-panel routing-panel">
        <div className="dashboard-section-head" style={{ marginBottom: '16px' }}>
          <div>
            <span className="eyebrow"><Settings2 size={12} /> Умная маршрутизация</span>
            <h2>Правила трафика</h2>
            <p className="muted">Соберите свой список маршрутов и добавьте их в приложение Happ одним нажатием.</p>
          </div>
        </div>
        
        {/* Tabs */}
        <div style={{ display: 'flex', gap: '8px', borderBottom: '1px solid rgba(255,255,255,0.1)', paddingBottom: '16px', marginBottom: '16px' }}>
          <button 
            className={`button ${activeTab === 'custom' ? 'button-secondary' : 'button-ghost'}`} 
            onClick={() => setActiveTab('custom')}
          >
            <Layers size={16} /> Мои маршруты
          </button>
          <button 
            className={`button ${activeTab === 'system' ? 'button-secondary' : 'button-ghost'}`} 
            onClick={() => setActiveTab('system')}
          >
            <Server size={16} /> Готовые пресеты
          </button>
        </div>
        
        {/* Happ Export Button */}
        {activeTab === 'custom' && (
          <div style={{ padding: '16px', background: 'rgba(234, 179, 8, 0.05)', borderRadius: '12px', border: '1px solid rgba(234, 179, 8, 0.2)', marginBottom: '24px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
            <div>
              <h3 style={{ color: 'var(--gold)', margin: '0 0 4px 0', fontSize: '15px' }}>Применить конфигурацию</h3>
              <p className="muted" style={{ margin: 0, fontSize: '13px' }}>У вас {customProfiles.length} правил(а). Отправьте их в приложение Happ.</p>
            </div>
            <button className="button button-primary gold-button" onClick={exportToHapp} disabled={customProfiles.length === 0}>
              <ExternalLink size={16} /> Добавить в Happ
            </button>
          </div>
        )}
        
        {/* Content */}
        {activeTab === 'custom' && (
          <div className="routing-profiles-list">
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '16px' }}>
              <button className="button button-ghost" onClick={startCreate} disabled={isEditing !== null}>
                <Plus size={16} /> Создать свое правило
              </button>
            </div>
            
            {isEditing !== null && (
              <div className="routing-editor" style={{ background: 'rgba(0,0,0,0.3)', padding: '20px', borderRadius: '12px', marginBottom: '16px', border: '1px solid var(--border)' }}>
                <h3>{isEditing === -1 ? "Новое правило" : "Редактировать правило"}</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', marginTop: '16px' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: 'var(--muted)' }}>Название</label>
                    <input 
                      className="vip-input" 
                      value={editName} 
                      onChange={e => setEditName(e.target.value)} 
                      placeholder="Например: YouTube или Заблокированные сайты" 
                      autoFocus 
                    />
                  </div>
                  
                  <div>
                    <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: 'var(--muted)' }}>Действие</label>
                    <select className="vip-input" value={editAction} onChange={e => setEditAction(e.target.value as any)}>
                      <option value="proxy">Через VPN (Proxy)</option>
                      <option value="direct">Напрямую (Direct)</option>
                    </select>
                  </div>
    
                  <div>
                    <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: 'var(--muted)' }}>Домены (через запятую, пробел или с новой строки)</label>
                    <textarea 
                      className="vip-input" 
                      style={{ minHeight: '100px', resize: 'vertical' }}
                      value={editDomains} 
                      onChange={e => setEditDomains(e.target.value)} 
                      placeholder="youtube.com\ngoogle.com" 
                    />
                  </div>
    
                  <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end', marginTop: '8px' }}>
                    <button className="button button-ghost" onClick={() => setIsEditing(null)} disabled={loading}>
                      <X size={16} /> Отмена
                    </button>
                    <button className="button button-primary gold-button" onClick={saveProfile} disabled={loading || !editName}>
                      <Check size={16} /> Сохранить
                    </button>
                  </div>
                </div>
              </div>
            )}
            
            {customProfiles.map(profile => (
              <div key={profile.id} className="routing-profile-item" style={{ padding: '16px', background: 'rgba(255,255,255,0.03)', borderRadius: '12px', marginBottom: '12px', border: '1px solid rgba(255,255,255,0.05)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
                <div className="profile-info" style={{ opacity: profile.enabled ? 1 : 0.5, transition: 'opacity 0.2s', minWidth: '200px', flex: '1 1 auto' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px', flexWrap: 'wrap' }}>
                    <strong>{profile.name}</strong>
                    <span style={{ fontSize: '10px', padding: '2px 6px', background: profile.action === 'proxy' ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)', color: profile.action === 'proxy' ? 'var(--green)' : 'var(--red)', borderRadius: '4px', textTransform: 'uppercase', fontWeight: 600 }}>
                      {profile.action}
                    </span>
                  </div>
                  <span className="muted" style={{ fontSize: '13px' }}>{profile.description || 'Пользовательское правило'}</span>
                </div>
                
                <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                  <label className="switch">
                    <input type="checkbox" checked={profile.enabled} onChange={() => toggleProfile(profile)} disabled={loading} />
                    <span className="slider"></span>
                  </label>
                  
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button className="button button-ghost" style={{ padding: '8px' }} onClick={() => startEdit(profile)} disabled={loading}>
                      <Edit2 size={16} />
                    </button>
                    <button className="button button-ghost" style={{ padding: '8px', color: 'var(--red)' }} onClick={() => deleteProfile(profile.id)} disabled={loading}>
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>
              </div>
            ))}
            
            {customProfiles.length === 0 && isEditing === null && (
              <div style={{ textAlign: 'center', padding: '40px 20px', background: 'rgba(0,0,0,0.2)', borderRadius: '12px' }}>
                <p className="muted" style={{ marginBottom: '16px' }}>У вас пока нет добавленных правил.</p>
                <button className="button button-secondary" onClick={() => setActiveTab('system')}>
                  Выбрать готовые пресеты
                </button>
              </div>
            )}
          </div>
        )}
        
        {activeTab === 'system' && (
          <div className="routing-profiles-list">
            {systemProfiles.map(profile => (
              <div key={profile.id} className="routing-profile-item" style={{ padding: '16px', background: 'rgba(255,255,255,0.03)', borderRadius: '12px', marginBottom: '12px', border: '1px solid rgba(255,255,255,0.05)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px', opacity: profile.already_added ? 0.5 : 1 }}>
                <div className="profile-info" style={{ minWidth: '200px', flex: '1 1 auto' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px', flexWrap: 'wrap' }}>
                    <strong>{profile.name}</strong>
                    <span style={{ fontSize: '10px', padding: '2px 6px', background: 'rgba(255,255,255,0.1)', borderRadius: '4px' }}>Системное</span>
                    <span style={{ fontSize: '10px', padding: '2px 6px', background: profile.action === 'proxy' ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)', color: profile.action === 'proxy' ? 'var(--green)' : 'var(--red)', borderRadius: '4px', textTransform: 'uppercase', fontWeight: 600 }}>
                      {profile.action}
                    </span>
                  </div>
                  <span className="muted" style={{ fontSize: '13px', display: 'block', maxWidth: '400px', lineHeight: 1.4 }}>{profile.description}</span>
                </div>
                
                <div>
                  {profile.already_added ? (
                    <span className="muted" style={{ fontSize: '13px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                      <Check size={14} /> Добавлено
                    </span>
                  ) : (
                    <button className="button button-secondary" onClick={() => copySystemProfile(profile.code)} disabled={loading}>
                      <Plus size={16} /> Добавить
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
