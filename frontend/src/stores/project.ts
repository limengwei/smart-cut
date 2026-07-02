import { create } from "zustand";
import type { Project } from "../api/types";

interface ProjectStore {
  currentProject: Project | null;
  setCurrentProject: (p: Project | null) => void;
}

export const useProjectStore = create<ProjectStore>((set) => ({
  currentProject: null,
  setCurrentProject: (p) => set({ currentProject: p }),
}));