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
    "Entropy-based routing reduces LLM inference costs by 91.6% while maintaining 98.2% accuracy. Measured on 518 prompts with real OpenAI APIs.",
  metadataBase: new URL("https://draft-thinker.vercel.app"),
  openGraph: {
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Entropy-based routing reduces LLM inference costs by 91.6% while maintaining 98.2% accuracy. Measured on 518 prompts with real OpenAI APIs.",
    url: "https://draft-thinker.vercel.app",
    siteName: "Draft-Thinker",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Draft-Thinker | Cost-Aware LLM Gateway",
    description:
      "Entropy-based routing reduces LLM inference costs by 91.6% while maintaining 98.2% accuracy.",
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
