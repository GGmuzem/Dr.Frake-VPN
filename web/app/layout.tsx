import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FBLink VPN",
  description: "Подписка, приложения и личный кабинет FBLink VPN.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
