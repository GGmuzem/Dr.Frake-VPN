"use client";

import { FormEvent, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, Eye, EyeOff, Mail, ShieldCheck } from "lucide-react";
import TravelConnectSignIn from "@/components/ui/travel-connect-signin";
import { Brand } from "./Brand";

type Mode = "login" | "register" | "reset";

const COPY: Record<Mode, { title: string; subtitle: string; cta: string }> = {
  login: {
    title: "С возвращением",
    subtitle: "Войдите, чтобы продлить подписку или скачать приложение.",
    cta: "Войти",
  },
  register: {
    title: "Создание аккаунта",
    subtitle: "Email — это ваш логин на всех устройствах. Пароль от 8 символов.",
    cta: "Продолжить",
  },
  reset: {
    title: "Сброс пароля",
    subtitle: "Введите email — пришлем код подтверждения.",
    cta: "Отправить код",
  },
};

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

  const buttonLabel = loading
    ? "Подождите..."
    : isCodeSent && mode !== "login"
    ? "Подтвердить"
    : COPY[mode].cta;

  return (
    <main className="auth-wrap">
      <TravelConnectSignIn>
        <div className="auth-main">
          <Brand />
          <span className="eyebrow auth-eyebrow">
            <ShieldCheck size={12} /> Шифрование AmneziaWG · Без логов
          </span>
          <h1>{COPY[mode].title}</h1>
          <p className="muted">{COPY[mode].subtitle}</p>
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
              <span className="form-label">
                <Mail size={14} /> Email
              </span>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                autoComplete="email"
                required
              />
            </label>
            {(mode !== "register" || !isCodeSent) && (
              <label>
                <span className="form-label">Пароль</span>
                <span className="password-control">
                  <input
                    type={isPasswordVisible ? "text" : "password"}
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder="от 8 символов"
                    autoComplete={mode === "login" ? "current-password" : "new-password"}
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
                <span className="form-label">Код из email</span>
                <input
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  inputMode="numeric"
                  pattern="[0-9]*"
                  placeholder="6-значный код"
                  required
                />
              </label>
            )}
            {message && <div className="notice">{message}</div>}
            {error && <div className="notice error">{error}</div>}
            <button className="button button-primary auth-submit" disabled={loading} type="submit">
              <span>{buttonLabel}</span>
              <ArrowRight size={16} />
            </button>
            <p className="auth-fineprint">
              Продолжая, вы соглашаетесь с условиями подписки. Возврат — в течение 7 дней.
            </p>
          </form>
        </div>
      </TravelConnectSignIn>
    </main>
  );
}
