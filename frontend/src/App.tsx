import { useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AppLayout } from "./layouts/AppLayout";
import { NewProject } from "./pages/NewProject";
import { Settings } from "./pages/Settings";
import { Workbench } from "./pages/Workbench";
import { useSettingsStore } from "./stores/settings";

function App() {
  const loadSettings = useSettingsStore((s) => s.loadSettings);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<Navigate to="/project/new" replace />} />
          <Route path="/project/new" element={<NewProject />} />
          <Route path="/project/:id" element={<Workbench />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;