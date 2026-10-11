import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export const metadata: Metadata = {
  title: "Draft-Thinker | Cost-Aware LLM Gateway",
  description:
    "Entropy-based routing cuts measured LLM inference cost by 82.9% (95% CI 79.6 to 86.0) at 98.1% judged accuracy on 519 prompts with real OpenAI APIs, with honest baselines.",
  metadataBase: new URL("https://draft-thinker.vercel.app"),
  openGraph: {
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Entropy-based routing cuts measured LLM inference cost by 82.9% (95% CI 79.6 to 86.0) at 98.1% judged accuracy on 519 prompts with real OpenAI APIs, with honest baselines.",
    url: "https://draft-thinker.vercel.app",
    siteName: "Draft-Thinker",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Entropy-based routing cuts measured LLM inference cost by 82.9% at 98.1% judged accuracy on 519 prompts.",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} font-sans antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
