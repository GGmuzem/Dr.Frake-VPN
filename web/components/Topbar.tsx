import { Download } from "lucide-react";
import { Brand } from "./Brand";

const NAV_LINKS = [
  { href: "#features", label: "Возможности" },
  { href: "#plans", label: "Тарифы" },
  { href: "#faq", label: "FAQ" },
];

export function Topbar() {
  return (
    <header className="topbar">
      <div className="container topbar-inner">
        <Brand />
        <nav className="topnav" aria-label="Главная навигация">
          {NAV_LINKS.map((link) => (
            <a key={link.href} href={link.href}>
              {link.label}
            </a>
          ))}
        </nav>
        <div className="nav-actions">
          <a className="button button-ghost" href="/auth">
            Войти
          </a>
          <a className="button button-primary" href="/dashboard">
            <Download size={16} /> Скачать
          </a>
        </div>
      </div>
    </header>
  );
}
