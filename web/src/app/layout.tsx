import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const themeInit =
  '(function(){var t=null;try{t=localStorage.getItem("theme")}catch(e){}if(t!=="light"&&t!=="dark"){t=window.matchMedia&&window.matchMedia("(prefers-color-scheme: light)").matches?"light":"dark"}document.documentElement.setAttribute("data-theme",t)})();';

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
    "Go LLM gateway with entropy-based routing, evaluated honestly: 82.9% measured cost reduction at 98.1% judged accuracy on 519 prompts, then replicated on GSM8K, MMLU and ARC-Challenge against random, oracle, mean-entropy and learned-router baselines, with speculative execution and semantic cache measured by replay.",
  metadataBase: new URL("https://draft-thinker.vercel.app"),
  openGraph: {
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Go LLM gateway with entropy-based routing, evaluated honestly: 82.9% measured cost reduction at 98.1% judged accuracy on 519 prompts, then replicated on GSM8K, MMLU and ARC-Challenge against random, oracle, mean-entropy and learned-router baselines, with speculative execution and semantic cache measured by replay.",
    url: "https://draft-thinker.vercel.app",
    siteName: "Draft-Thinker",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Entropy-based LLM routing in Go: 82.9% measured cost reduction at 98.1% accuracy, replicated on three public benchmarks with honest baselines.",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} font-sans antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
