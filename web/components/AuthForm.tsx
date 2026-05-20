"use client";

import { FormEvent, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, Eye, EyeOff } from "lucide-react";
import TravelConnectSignIn from "@/components/ui/travel-connect-signin";
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
  const [isPasswordVisible, setIsPasswordVisible] = useState(false);
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
      <TravelConnectSignIn>
        <div className="auth-main">
          <Brand />
          <h1>Войти в FBLink VPN</h1>
          <p className="muted">Оплата, продление и скачивание приложений собраны в одном кабинете.</p>
          <div className="tabs">
            <button className={mode === "login" ? "active" : ""} onClick={() => switchMode("login")} type="button">
              Вход
            </button>
            <button className={mode === "register" ? "active" : ""} onClick={() => switchMode("register")} type="button">
              Регистрация
            </button>
            <button className={mode === "reset" ? "active" : ""} onClick={() => switchMode("reset")} type="button">
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
                <span className="password-control">
                  <input
                    type={isPasswordVisible ? "text" : "password"}
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    minLength={8}
                    required
                  />
                  <button
                    aria-label={isPasswordVisible ? "Скрыть пароль" : "Показать пароль"}
                    className="password-toggle"
                    onClick={() => setIsPasswordVisible((value) => !value)}
                    type="button"
                  >
                    {isPasswordVisible ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </span>
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
              <span>{loading ? "Подождите..." : mode === "login" ? "Войти" : isCodeSent ? "Подтвердить" : "Продолжить"}</span>
              <ArrowRight size={16} />
            </button>
          </form>
        </div>
      </TravelConnectSignIn>
    </main>
  );
}
