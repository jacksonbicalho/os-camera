import AppLayout from "./AppLayout";
import { SettingsSidebar } from "./SettingsSidebar";
import { getRole } from "../auth";

const BASE_NAV_LINKS = [
  { to: "/settings/cameras", label: "Câmeras" },
  { to: "/settings/discover", label: "Rastrear câmeras" },
  { to: "/settings/users", label: "Usuários" },
  { to: "/settings/server", label: "Servidor" },
  { to: "/settings/storage", label: "Armazenamento" },
  { to: "/settings/system", label: "Sistema" },
  { to: "/settings/about", label: "Sobre" },
];

const VIEWER_NAV_LINKS = BASE_NAV_LINKS.filter(l => l.to !== "/settings/users");

interface SettingsLayoutProps {
  children: React.ReactNode;
}

export default function SettingsLayout({ children }: SettingsLayoutProps) {
  const navLinks = getRole() === "admin" ? BASE_NAV_LINKS : VIEWER_NAV_LINKS;

  return (
    <AppLayout mainClassName="max-w-4xl mx-auto w-full">
      <h2 className="text-2xl font-bold text-white mb-6">Configurações</h2>
      <div className="flex gap-10">
        <SettingsSidebar NAV_LINKS={navLinks} />
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </AppLayout>
  );
}
