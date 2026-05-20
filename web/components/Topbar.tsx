import { Download } from "lucide-react";
import { Brand } from "./Brand";

export function Topbar() {
  return (
    <header className="topbar">
      <div className="container topbar-inner">
        <Brand />
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
