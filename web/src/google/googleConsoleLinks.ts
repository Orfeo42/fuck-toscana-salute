export const googleConsoleLinks = {
  dashboard: "https://console.cloud.google.com/apis/dashboard",
  library: "https://console.cloud.google.com/apis/library",
  credentials: "https://console.cloud.google.com/apis/credentials",
  clients: "https://console.cloud.google.com/auth/clients",
  scopes: "https://console.cloud.google.com/auth/scopes",
  audience: "https://console.cloud.google.com/auth/audience",
} as const;

export const googleConsoleLinkLabels: readonly {
  key: keyof typeof googleConsoleLinks;
  label: string;
}[] = [
  { key: "dashboard", label: "API abilitate" },
  { key: "library", label: "Libreria API" },
  { key: "credentials", label: "Credenziali (API key)" },
  { key: "clients", label: "Client OAuth" },
  { key: "scopes", label: "Accesso ai dati (scope)" },
  { key: "audience", label: "Pubblico e utenti di test" },
];
