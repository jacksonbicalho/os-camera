import * as React from "react"

import { cn } from "@/lib/utils"

// Label simples (sem @radix-ui/react-label, para não adicionar dependência) —
// estilo shadcn, usa os tokens do tema.
const Label = React.forwardRef<HTMLLabelElement, React.LabelHTMLAttributes<HTMLLabelElement>>(
  ({ className, ...props }, ref) => (
    <label
      ref={ref}
      className={cn(
        "text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
        className,
      )}
      {...props}
    />
  ),
)
Label.displayName = "Label"

export { Label }
