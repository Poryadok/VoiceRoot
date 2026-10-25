import eslint from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default tseslint.config(
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: {
      "react-hooks": reactHooks,
    },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
  {
    ignores: ["dist/**"],
  },
  {
    files: ["browser/**/*.mjs"],
    languageOptions: {
      globals: {
        process: "readonly", Buffer: "readonly", WebSocket: "readonly",
        URL: "readonly", URLSearchParams: "readonly", console: "readonly", fetch: "readonly",
        setTimeout: "readonly", clearTimeout: "readonly",
      },
    },
  },
);
