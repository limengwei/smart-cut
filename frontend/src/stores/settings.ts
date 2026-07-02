import { create } from "zustand";
import { getSettings, saveSettings } from "../api/client";
import type { GlobalSettings } from "../api/types";

interface SettingsStore {
  settings: GlobalSettings | null;
  loading: boolean;
  loadSettings: () => Promise<void>;
  updateSettings: (s: GlobalSettings) => Promise<void>;
}

export const useSettingsStore = create<SettingsStore>((set) => ({
  settings: null,
  loading: false,

  loadSettings: async () => {
    set({ loading: true });
    try {
      const settings = await getSettings();
      set({ settings, loading: false });
      applyTheme(settings.theme);
    } catch (e) {
      set({ loading: false });
      console.error("加载设置失败:", e);
    }
  },

  updateSettings: async (s: GlobalSettings) => {
    await saveSettings(s);
    set({ settings: s });
    applyTheme(s.theme);
  },
}));

function applyTheme(theme: string) {
  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
}