import { NavLink } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { useState } from "react";

type NavLinkItem = {
  to: string;
  label: string;
};

type SettingsSidebarProps = {
  NAV_LINKS: NavLinkItem[];
};

export function SettingsSidebar({ NAV_LINKS }: SettingsSidebarProps) {
  const [openSidebar, setOpenSidebar] = useState(false);

  return (
    <>
      {/* Overlay */}
      {openSidebar && (
        <div
          className="fixed inset-0 z-40 bg-black/50 opacity-65 md:hidden"
          onClick={() => setOpenSidebar(false)}
        />
      )}

      {/* Handle mobile */}
      <button
        onClick={() => setOpenSidebar((prev) => !prev)}
        className={`
    fixed top-1/2 -translate-y-1/2
    z-60 md:hidden
    flex items-center justify-center

    h-24 w-5
    rounded-r-2xl
    bg-[#6b7d98]
    opacity-80
    shadow-lg shadow-violet-900/40

    border border-violet-400/30
    border-l-0

    text-white

    transition-all duration-300
    active:scale-95

    ${openSidebar ? "left-72" : "left-0"}
  `}
      >
        <ChevronRight
          size={22}
          strokeWidth={2.5}
          className={`
      transition-transform duration-300
      ${openSidebar ? "rotate-180" : ""}
    `}
        />
      </button>

      {/* Sidebar */}
      <aside
        className={`
    fixed top-0 left-0 z-50
    h-screen w-72
    bg-black
    border-r border-gray-800
    p-4
    transition-transform duration-300
    md:static md:h-auto md:w-44 md:border-none md:bg-transparent md:p-0

    ${openSidebar ? "translate-x-0" : "-translate-x-full"}

    md:translate-x-0
  `}
      >
        <nav className="mt-8 md:mt-0 w-full">
          <p className="mb-3 text-xs font-medium uppercase tracking-wider text-gray-500">
            Configurações
          </p>

          <ul className="flex flex-col gap-1">
            {NAV_LINKS.map(({ to, label }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  onClick={() => setOpenSidebar(false)}
                  className={({ isActive }: { isActive: boolean }) =>
                    `
                      flex items-center rounded-lg px-3 py-2
                      text-sm transition-colors
                      ${
                        isActive
                          ? "bg-gray-800 text-white font-medium"
                          : "text-gray-400 hover:bg-gray-800 hover:text-white"
                      }
                    `
                  }
                >
                  {label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
      </aside>
    </>
  );
}
