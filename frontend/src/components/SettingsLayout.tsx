import AppLayout from "./AppLayout";
import { SettingsSidebar } from "./SettingsSidebar";
import { getRole } from "../auth";

const BASE_NAV_LINKS = [
  { to: "/settings/cameras", label: "Câmeras" },
  { to: "/settings/discover", label: "Rastrear câmeras" },
  { to: "/settings/users", label: "Usuários" },
  { to: "/settings/server", label: "Servidor" },
  { to: "/settings/storage", label: "Armazenamento" },
  { to: "/settings/analysis", label: "Análise de vídeo" },
  { to: "/settings/system", label: "Sistema" },
  { to: "/settings/appearance", label: "Aparência" },
  { to: "/settings/about", label: "Sobre" },
];

const VIEWER_NAV_LINKS = BASE_NAV_LINKS.filter(l =>
  l.to === "/settings/cameras" || l.to === "/settings/appearance" || l.to === "/settings/about"
);

interface SettingsLayoutProps {
  children: React.ReactNode;
}

export default function SettingsLayout({ children }: SettingsLayoutProps) {
  const navLinks = getRole() === "admin" ? BASE_NAV_LINKS : VIEWER_NAV_LINKS;

  return (
    <AppLayout mainClassName="w-full h-full flex flex-col">
      <h2 className="text-2xl font-bold text-white mb-6 shrink-0">Configurações</h2>
      <div className="flex gap-10 flex-1 min-h-0">
        <SettingsSidebar NAV_LINKS={navLinks} />
        <div className="flex-1 min-w-0 overflow-y-auto pb-6">{children}</div>
      </div>
    </AppLayout>
  );
}
