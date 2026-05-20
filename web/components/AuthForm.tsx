"use client";

import { FormEvent, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Brand } from "./Brand";

type Mode = "login" | "register" | "reset";

async function postJSON(path: string, body: unknown) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error ?? data.message ?? "Не удалось выполнить запрос");
  }
  return data;
}

export function AuthForm() {
  const router = useRouter();
  const params = useSearchParams();
  const initialPlan = params.get("plan") ?? "";
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isCodeSent, setIsCodeSent] = useState(false);
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setMessage("");
    setLoading(true);
    try {
      if (mode === "login") {
        await postJSON("/api/auth/login", { email, password });
        router.push(initialPlan ? `/dashboard?plan=${initialPlan}` : "/dashboard");
      } else if (mode === "register" && !isCodeSent) {
        await postJSON("/api/auth/register", { email, password });
        setIsCodeSent(true);
        setMessage("Код подтверждения отправлен на email.");
      } else if (mode === "register") {
        await postJSON("/api/auth/verify", { email, code });
        router.push(initialPlan ? `/dashboard?plan=${initialPlan}` : "/dashboard");
      } else if (mode === "reset" && !isCodeSent) {
        await postJSON("/api/auth/forgot", { email });
        setIsCodeSent(true);
        setMessage("Если email зарегистрирован, код уже отправлен.");
      } else {
        await postJSON("/api/auth/reset", { email, code, new_password: password });
        setMode("login");
        setIsCodeSent(false);
        setMessage("Пароль обновлен. Войдите с новым паролем.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка запроса");
    } finally {
      setLoading(false);
    }
  }

  function switchMode(nextMode: Mode) {
    setMode(nextMode);
    setIsCodeSent(false);
    setMessage("");
    setError("");
  }

  return (
    <main className="auth-wrap">
      <section className="auth-card">
        <Brand />
        <h1 style={{ marginTop: 24 }}>Личный кабинет</h1>
        <p className="muted">Войдите, купите подписку или подтвердите новый аккаунт.</p>
        <div className="tabs">
          <button className={mode === "login" ? "active" : ""} onClick={() => switchMode("login")}>
            Вход
          </button>
          <button className={mode === "register" ? "active" : ""} onClick={() => switchMode("register")}>
            Регистрация
          </button>
          <button className={mode === "reset" ? "active" : ""} onClick={() => switchMode("reset")}>
            Пароль
          </button>
        </div>
        <form className="form" onSubmit={onSubmit}>
          <label>
            Email
            <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} required />
          </label>
          {(mode !== "register" || !isCodeSent) && (
            <label>
              Пароль
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                minLength={8}
                required
              />
            </label>
          )}
          {isCodeSent && (
            <label>
              Код из email
              <input value={code} onChange={(event) => setCode(event.target.value)} required />
            </label>
          )}
          {message && <div className="notice">{message}</div>}
          {error && <div className="notice error">{error}</div>}
          <button className="button button-primary" disabled={loading} type="submit">
            {loading ? "Подождите..." : mode === "login" ? "Войти" : isCodeSent ? "Подтвердить" : "Продолжить"}
          </button>
        </form>
      </section>
    </main>
  );
}
