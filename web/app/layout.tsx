import type { Metadata } from "next";
import { DM_Sans, Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const display = DM_Sans({
  subsets: ["latin", "latin-ext"],
  variable: "--font-display",
  weight: ["400", "500", "600", "700", "800", "900"],
  display: "swap",
});

const body = Inter({
  subsets: ["latin", "cyrillic"],
  variable: "--font-body",
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const mono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  weight: ["400", "500"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "FBLink VPN — приватный доступ за минуту",
  description:
    "Премиальный VPN без логов. Приложения для Android, Windows, macOS, Linux и iPhone. Подписка Premium или VIP, понятный личный кабинет.",
  metadataBase: new URL("https://fblink-sc.com"),
  openGraph: {
    title: "FBLink VPN",
    description: "Премиальный VPN без логов. Подписка Premium или VIP за пару минут.",
    type: "website",
    locale: "ru_RU",
  },
  icons: {
    icon: "/brand-icon.png",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" className={`${display.variable} ${body.variable} ${mono.variable}`}>
      <body>{children}</body>
    </html>
  );
}
