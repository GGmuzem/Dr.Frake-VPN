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
          <a className="button button-primary" href="/#plans">
            Купить подписку
          </a>
        </div>
      </div>
    </header>
  );
}
