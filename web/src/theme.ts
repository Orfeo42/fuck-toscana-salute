import { AppShell, createTheme } from "@mantine/core";

const surface =
  "light-dark(var(--mantine-color-white), var(--mantine-color-dark-6))";

export const tableBackground = { light: "white", dark: "dark.6" } as const;

export const theme = createTheme({
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
  components: {
    AppShell: AppShell.extend({
      styles: {
        main: {
          backgroundColor:
            "light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-8))",
        },
        header: {
          backgroundColor: surface,
          boxShadow: "var(--mantine-shadow-xs)",
        },
        navbar: { backgroundColor: surface },
      },
    }),
  },
});
