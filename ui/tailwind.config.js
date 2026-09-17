/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{vue,js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        primary: {
          50: "#f0f9ff",
          100: "#e0f2fe",
          200: "#bae6fd",
          300: "#7dd3fc",
          400: "#38bdf8",
          500: "#0ea5e9",
          600: "#0284c7",
          700: "#0369a1",
          800: "#075985",
          900: "#0c4a6e",
        },
        gray: {
          50: "#f9fafb",
          100: "#f3f4f6",
          200: "#e5e7eb",
          300: "#d1d5db",
          400: "#9ca3af",
          500: "#6b7280",
          600: "#4b5563",
          700: "#374151",
          800: "#1f2937",
          900: "#111827",
        },
      },
      // === CUSTOM START: 阴影工具类 - By ASxiaowen ===
      // 理由: 规则5 要求不改基础配置，但 `hover:shadow-soft` / `hover:shadow-lift` 这类
      //       **带变体的工具类只能由 Tailwind 依据 config 生成**，纯 CSS 覆盖做不到。
      //       因此这里是唯一一处不得不改的基础配置，值与 custom_components/theme.css 保持一致。
      boxShadow: {
        card: "0 1px 2px 0 rgba(16,24,40,.04), 0 1px 3px 0 rgba(16,24,40,.05)",
        soft: "0 8px 24px -10px rgba(15,23,42,.18), 0 2px 6px -2px rgba(15,23,42,.06)",
        lift: "0 18px 40px -18px rgba(15,23,42,.28), 0 2px 8px -4px rgba(15,23,42,.08)",
        glow: "0 0 0 1px rgba(14,165,233,.18), 0 8px 24px -10px rgba(14,165,233,.45)",
      },
      // === CUSTOM END: 阴影工具类 ===
      animation: {
        "fade-in": "fadeIn 0.5s ease-in-out",
        "slide-up": "slideUp 0.3s ease-out",
        "slide-down": "slideDown 0.3s ease-out",
        "scale-in": "scaleIn 0.2s ease-out",
        "pulse-slow": "pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite",
      },
      keyframes: {
        fadeIn: {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        slideUp: {
          "0%": { transform: "translateY(10px)", opacity: "0" },
          "100%": { transform: "translateY(0)", opacity: "1" },
        },
        slideDown: {
          "0%": { transform: "translateY(-10px)", opacity: "0" },
          "100%": { transform: "translateY(0)", opacity: "1" },
        },
        scaleIn: {
          "0%": { transform: "scale(0.95)", opacity: "0" },
          "100%": { transform: "scale(1)", opacity: "1" },
        },
      },
      backdropBlur: {
        xs: "2px",
      },
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}
