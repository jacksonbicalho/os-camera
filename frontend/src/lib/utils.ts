import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

// cn — junta classes condicionais (clsx) e resolve conflitos de Tailwind
// (tailwind-merge). Base dos componentes shadcn/ui.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
