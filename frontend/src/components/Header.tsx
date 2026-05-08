import { useState, useRef, useEffect } from "react";
import { useNavigate, NavLink, Link } from "react-router-dom";
import { clearToken } from "../auth";

interface HeaderProps {
  username?: string;
}

function useDropdown() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);
  return { open, setOpen, ref };
}

export default function Header({ username = "usuário" }: HeaderProps) {
  const { open: userOpen, setOpen: setUserOpen, ref: userRef } = useDropdown();

  const navigate = useNavigate();

  function logout() {
    clearToken();
    navigate("/login");
  }

  return (
    <header className="flex items-center justify-between px-6 py-3 bg-gray-900 border-b border-gray-800">
      <Link
        to="/"
        className="text-white font-semibold tracking-wide hover:text-gray-200 transition-colors"
      >
        📷 Camera
      </Link>

      <div className="flex items-center gap-4">
        <NavLink
          to="/settings/stats"
          className={({ isActive }) =>
            `text-sm transition-colors ${isActive || location.pathname.startsWith("/settings") ? "text-white" : "text-gray-400 hover:text-white"}`
          }
        >
          Configurações
        </NavLink>

        <div className="relative" ref={userRef}>
          <button
            onClick={() => setUserOpen((v) => !v)}
            className="flex items-center gap-2 text-sm text-gray-300 hover:text-white transition-colors"
          >
            {username}
            <svg
              className={`w-4 h-4 transition-transform ${userOpen ? "rotate-180" : ""}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </button>
          {userOpen && (
            <div className="absolute right-0 mt-2 w-36 bg-gray-800 border border-gray-700 rounded shadow-lg z-10">
              <button
                onClick={logout}
                className="block w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
              >
                Sair
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
