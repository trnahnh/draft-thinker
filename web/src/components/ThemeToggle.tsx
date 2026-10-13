"use client";

import { useEffect } from "react";
import { Moon, Sun } from "lucide-react";

const STORAGE_KEY = "theme";

type Theme = "dark" | "light";

function apply(theme: Theme) {
  document.documentElement.setAttribute("data-theme", theme);
}

function current(): Theme {
  return document.documentElement.getAttribute("data-theme") === "light"
    ? "light"
    : "dark";
}

export default function ThemeToggle() {
  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key !== STORAGE_KEY) return;
      apply(e.newValue === "light" ? "light" : "dark");
    }
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  function toggle() {
    const next: Theme = current() === "light" ? "dark" : "light";
    apply(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Storage can be unavailable (private mode, blocked site data).
      // The page still switches for this view.
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Toggle color theme"
      title="Toggle color theme"
      className="flex h-6 w-6 items-center justify-center border border-(--color-border) text-(--color-text-dark) transition-colors hover:border-(--color-border-bright) hover:text-(--color-text) focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-(--color-cyan)"
    >
      <Sun className="theme-icon-light h-3.5 w-3.5" aria-hidden="true" />
      <Moon className="theme-icon-dark h-3.5 w-3.5" aria-hidden="true" />
    </button>
  );
}
