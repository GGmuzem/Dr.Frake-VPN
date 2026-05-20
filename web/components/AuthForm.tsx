"use client";

import { FormEvent, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowLeft, ArrowRight, Eye, EyeOff, Mail, ShieldCheck } from "lucide-react";
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
    subtitle: "Email станет вашим логином на всех устройствах. Пароль от 8 символов.",
    cta: "Создать аккаунт",
  },
  reset: {
    title: "Восстановление пароля",
    subtitle: "Введите email — пришлем код для сброса пароля.",
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
    setCode("");
  }

  const showPassword = mode === "login" || (mode === "register" && !isCodeSent) || (mode === "reset" && isCodeSent);
  const showCode = isCodeSent && mode !== "login";

  const buttonLabel = loading ? "Подождите..." : showCode ? "Подтвердить" : COPY[mode].cta;

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

            {showPassword && (
              <div className="field">
                <div className="form-label-row">
                  <label className="form-label" htmlFor="auth-password">
                    {mode === "reset" ? "Новый пароль" : "Пароль"}
                  </label>
                  {mode === "login" && (
                    <button
                      className="link-button"
                      onClick={() => switchMode("reset")}
                      type="button"
                    >
                      Забыли пароль?
                    </button>
                  )}
                </div>
                <span className="password-control">
                  <input
                    id="auth-password"
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
              </div>
            )}

            {showCode && (
              <label>
                <span className="form-label">Код из email</span>
                <input
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  inputMode="numeric"
                  pattern="[0-9]*"
                  placeholder="6-значный код"
                  autoComplete="one-time-code"
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

          </form>

          <div className="auth-switch">
            {mode === "login" && (
              <p>
                Нет аккаунта?{" "}
                <button className="link-button" onClick={() => switchMode("register")} type="button">
                  Создать
                </button>
              </p>
            )}
            {mode === "register" && (
              <p>
                Уже есть аккаунт?{" "}
                <button className="link-button" onClick={() => switchMode("login")} type="button">
                  Войти
                </button>
              </p>
            )}
            {mode === "reset" && (
              <p>
                <button className="link-button link-button-with-icon" onClick={() => switchMode("login")} type="button">
                  <ArrowLeft size={14} /> Назад ко входу
                </button>
              </p>
            )}
          </div>

          <p className="auth-fineprint">
            Продолжая, вы соглашаетесь с условиями подписки. Без автопродления — оплата вручную.
          </p>
        </div>
      </TravelConnectSignIn>
    </main>
  );
}
