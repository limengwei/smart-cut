import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Settings, FolderPlus, Scissors, Home } from "lucide-react";
import { cn } from "../lib/utils";

export function AppLayout() {
  const navigate = useNavigate();

  const navItems = [
    { to: "/", label: "首页", icon: Home },
    { to: "/project/new", label: "新建项目", icon: FolderPlus },
    { to: "/settings", label: "设置", icon: Settings },
  ];

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      <aside className="flex w-16 flex-col items-center gap-4 border-r border-border bg-zinc-900 py-4">
        <div
          className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-lg bg-primary text-primary-foreground"
          onClick={() => navigate("/")}
          title="Smart-Cut"
        >
          <Scissors className="h-5 w-5" />
        </div>

        <nav className="flex flex-1 flex-col gap-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "flex h-10 w-10 items-center justify-center rounded-lg transition-colors",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                )
              }
              title={item.label}
            >
              <item.icon className="h-5 w-5" />
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}