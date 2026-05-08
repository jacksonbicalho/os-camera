import AppLayout from "./AppLayout";
import { SettingsSidebar } from "./SettingsSidebar";

const NAV_LINKS = [
  { to: "/settings/stats", label: "Estatísticas" },
  { to: "/settings/cameras", label: "Câmeras" },
  { to: "/settings/server", label: "Servidor" },
  { to: "/settings/storage", label: "Armazenamento" },
  { to: "/settings/system", label: "Sistema" },
  { to: "/settings/about", label: "Sobre" },
];

interface SettingsLayoutProps {
  children: React.ReactNode;
}

export default function SettingsLayout({ children }: SettingsLayoutProps) {
  return (
    <AppLayout mainClassName="max-w-5xl mx-auto w-full">
      <div className="flex gap-8">
        <SettingsSidebar NAV_LINKS={NAV_LINKS} />
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </AppLayout>
  );
}
